package x509utils

// Copyright (c) 2011-2015 Michael Mitton (mmitton@gmail.com)
// Portions copyright (c) 2015-2016 go-ldap Authors
//
// From https://github.com/go-ldap/ldap/blob/master/dn.go
// Lisense: MIT

import (
	"bytes"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/cesanta/errors"
)

var attributeTypeNames = map[string]asn1.ObjectIdentifier{
	"CN":           asn1.ObjectIdentifier{2, 5, 4, 3},
	"SERIALNUMBER": asn1.ObjectIdentifier{2, 5, 4, 5},
	"C":            asn1.ObjectIdentifier{2, 5, 4, 6},
	"L":            asn1.ObjectIdentifier{2, 5, 4, 7},
	"ST":           asn1.ObjectIdentifier{2, 5, 4, 8},
	"STREET":       asn1.ObjectIdentifier{2, 5, 4, 9},
	"O":            asn1.ObjectIdentifier{2, 5, 4, 10},
	"OU":           asn1.ObjectIdentifier{2, 5, 4, 11},
	"POSTALCODE":   asn1.ObjectIdentifier{2, 5, 4, 17},
}

// AttributeTypeAndValue represents an attributeTypeAndValue from https://tools.ietf.org/html/rfc4514
type AttributeTypeAndValue struct {
	// Type is the attribute type
	Type string
	// Value is the attribute value
	Value string
}

// RelativeDN represents a relativeDistinguishedName from https://tools.ietf.org/html/rfc4514
type RelativeDN struct {
	Attributes []*AttributeTypeAndValue
}

// DN represents a distinguishedName from https://tools.ietf.org/html/rfc4514
type DN struct {
	RDNs []*RelativeDN
}

// ParseDN returns a distinguishedName or an error
func ParseDN(str string) (*DN, error) {
	dn := new(DN)
	dn.RDNs = make([]*RelativeDN, 0)
	rdn := new(RelativeDN)
	rdn.Attributes = make([]*AttributeTypeAndValue, 0)
	buffer := bytes.Buffer{}
	attribute := new(AttributeTypeAndValue)
	escaping := false

	unescapedTrailingSpaces := 0
	stringFromBuffer := func() string {
		s := buffer.String()
		s = s[0 : len(s)-unescapedTrailingSpaces]
		buffer.Reset()
		unescapedTrailingSpaces = 0
		return s
	}

	for i := 0; i < len(str); i++ {
		char := str[i]
		switch {
		case escaping:
			unescapedTrailingSpaces = 0
			escaping = false
			switch char {
			case ' ', '"', '#', '+', ',', ';', '<', '=', '>', '\\':
				buffer.WriteByte(char)
				continue
			}
			// Not a special character, assume hex encoded octet
			if len(str) == i+1 {
				return nil, errors.New("got corrupted escaped character")
			}

			dst := []byte{0}
			n, err := hex.Decode([]byte(dst), []byte(str[i:i+2]))
			if err != nil {
				return nil, fmt.Errorf("failed to decode escaped character: %s", err)
			} else if n != 1 {
				return nil, fmt.Errorf("expected 1 byte when un-escaping, got %d", n)
			}
			buffer.WriteByte(dst[0])
			i++
		case char == '\\':
			unescapedTrailingSpaces = 0
			escaping = true
		case char == '=':
			attribute.Type = stringFromBuffer()
			// Special case: If the first character in the value is # the
			// following data is BER encoded so we can just fast forward
			// and decode.
			if len(str) > i+1 && str[i+1] == '#' {
				return nil, errors.New("BER values not supported")
				/*
					i += 2
					index := strings.IndexAny(str[i:], ",+")
					data := str
					if index > 0 {
						data = str[i : i+index]
					} else {
						data = str[i:]
					}
						rawBER, err := enchex.DecodeString(data)
						if err != nil {
							return nil, fmt.Errorf("failed to decode BER encoding: %s", err)
						}
							packet, err := ber.DecodePacketErr(rawBER)
							if err != nil {
								return nil, fmt.Errorf("failed to decode BER packet: %s", err)
							}
							buffer.WriteString(packet.Data.String())
							i += len(data) - 1
				*/
			}
		case char == ',' || char == '+':
			// We're done with this RDN or value, push it
			if len(attribute.Type) == 0 {
				return nil, errors.New("incomplete type, value pair")
			}
			attribute.Value = stringFromBuffer()
			rdn.Attributes = append(rdn.Attributes, attribute)
			attribute = new(AttributeTypeAndValue)
			if char == ',' {
				dn.RDNs = append(dn.RDNs, rdn)
				rdn = new(RelativeDN)
				rdn.Attributes = make([]*AttributeTypeAndValue, 0)
			}
		case char == ' ' && buffer.Len() == 0:
			// ignore unescaped leading spaces
			continue
		default:
			if char == ' ' {
				// Track unescaped spaces in case they are trailing and we need to remove them
				unescapedTrailingSpaces++
			} else {
				// Reset if we see a non-space char
				unescapedTrailingSpaces = 0
			}
			buffer.WriteByte(char)
		}
	}
	if buffer.Len() > 0 {
		if len(attribute.Type) == 0 {
			return nil, errors.New("DN ended with incomplete type, value pair")
		}
		attribute.Value = stringFromBuffer()
		rdn.Attributes = append(rdn.Attributes, attribute)
		dn.RDNs = append(dn.RDNs, rdn)
	}
	return dn, nil
}

func (dn *DN) ToPKIXName() (*pkix.Name, error) {
	var ns pkix.RDNSequence
	for _, rdn := range dn.RDNs {
		var s pkix.RelativeDistinguishedNameSET
		for _, a := range rdn.Attributes {
			oid, ok := attributeTypeNames[a.Type]
			if !ok {
				for _, p := range strings.Split(a.Type, ".") {
					pi, err := strconv.Atoi(p)
					if err != nil {
						return nil, errors.Errorf("invalid attribute type %q", a.Type)
					}
					oid = append(oid, pi)
				}
			}
			s = append(s, pkix.AttributeTypeAndValue{
				Type:  oid,
				Value: a.Value,
			})
		}
		ns = append(ns, s)
	}
	var n pkix.Name
	n.FillFromRDNSequence(&ns)
	return &n, nil
}
