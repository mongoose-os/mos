package fwbundle

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	"github.com/cesanta/errors"
)

type FirmwarePart firmwarePart

type firmwarePart struct {
	Name           string `json:"-"`
	Type           string `json:"type,omitempty"`
	Src            string `json:"src,omitempty"`
	Addr           uint32 `json:"addr,omitempty"`
	Size           uint32 `json:"size,omitempty"`
	Fill           *uint8 `json:"fill,omitempty"`
	ChecksumSHA1   string `json:"cs_sha1,omitempty"`
	ChecksumSHA256 string `json:"cs_sha256,omitempty"`
	// For SPIFFS images.
	FSSize      uint32 `json:"fs_size,omitempty"`
	FSBlockSize uint32 `json:"fs_block_size,omitempty"`
	FSEraseSize uint32 `json:"fs_erase_size,omitempty"`
	FSPageSize  uint32 `json:"fs_page_size,omitempty"`
	// Platform-specific stuff:
	// ESP32, ESP8266
	ESP32Encrypt       bool   `json:"encrypt,omitempty"`
	ESP32PartitionName string `json:"ptn,omitempty"`
	// CC32xx
	CC32XXFileAllocSize int    `json:"falloc,omitempty"`
	CC32XXFileSignature string `json:"sig,omitempty"`
	CC32XXSigningCert   string `json:"sig_cert,omitempty"`
	// Deprecated, not being used since 2018/12/03.
	CC3200FileSignature string `json:"sign,omitempty"`

	// Other user-specified properties are preserved here.
	properties   map[string]interface{}
	data         []byte
	dataProvider DataProvider
}

type DataProvider func(name, src string) ([]byte, error)

func PartFromString(ps string) (*FirmwarePart, error) {
	np := strings.SplitN(ps, ":", 2)
	if len(np) < 2 {
		return nil, errors.Errorf("invalid part spec '%s', must be 'name:prop=value,...'", ps)
	}
	// Create properties JSON and re-parse it.
	m := make(map[string]interface{})
	for _, prop := range strings.Split(np[1], ",") {
		if len(prop) == 0 {
			break
		}
		kv := strings.SplitN(prop, "=", 2)
		if len(kv) < 2 {
			return nil, errors.Errorf("invalid property spec '%s', must be 'prop=value'", prop)
		}
		k := kv[0]
		v := kv[1]
		switch {
		case v == "":
			m[k] = ""
		case v == "true":
			m[k] = true
		case v == "false":
			m[k] = false
		case string(v[0]) == `'`: // Sinly-quoted string
			m[k] = strings.Replace(v[1:len(v)-1], `\'`, `'`, -1)
		case string(v[0]) == `"`: // Double-quoted string
			m[k] = strings.Replace(v[1:len(v)-1], `\"`, `"`, -1)
		default:
			if n, nerr := strconv.ParseInt(v, 0, 32); nerr == nil {
				m[k] = n
			} else {
				m[k] = v // Simple unquoted string
			}
		}
	}
	mb, _ := json.Marshal(&m)
	var p FirmwarePart
	if err := json.Unmarshal(mb, &p); err != nil {
		return nil, err
	}
	p.Name = np[0]
	return &p, nil
}

func computeSHA1(data []byte) string {
	csSHA1 := sha1.Sum(data)
	return strings.ToLower(hex.EncodeToString(csSHA1[:]))
}

func computeSHA256(data []byte) string {
	csSHA256 := sha256.Sum256(data)
	return strings.ToLower(hex.EncodeToString(csSHA256[:]))
}

func (p *FirmwarePart) CalcChecksum() error {
	data, err := p.GetData()
	if err != nil {
		return errors.Trace(err)
	}
	p.ChecksumSHA1 = computeSHA1(data)
	p.ChecksumSHA256 = computeSHA256(data)
	return nil
}

func (p *FirmwarePart) SetData(data []byte) {
	p.data = data[:]
	p.CalcChecksum()
}

func (p *FirmwarePart) SetDataProvider(dp DataProvider) {
	p.dataProvider = dp
}

func (p *FirmwarePart) GetData() ([]byte, error) {
	var data []byte
	var err error
	if p.Src != "" {
		if p.data != nil {
			data = p.data[:]
		} else {
			if p.dataProvider != nil {
				data, err = p.dataProvider(p.Name, p.Src)
				if err != nil {
					return nil, errors.Annotatef(err, "%s: error retrieving data", p.Name)
				}
			} else {
				return nil, errors.Errorf("%s: no suitable data source", p.Name)
			}
		}
		if p.ChecksumSHA1 != "" {
			csSHA1 := computeSHA1(data)
			if p.ChecksumSHA1 != csSHA1 {
				return nil, errors.Errorf("%s: checksum does not match (want %s, got %s)", p.Name, p.ChecksumSHA1, csSHA1)
			}
		}
		if p.ChecksumSHA256 != "" {
			csSHA256 := computeSHA256(data)
			if p.ChecksumSHA256 != csSHA256 {
				return nil, errors.Errorf("%s: checksum does not match (want %s, got %s)", p.Name, p.ChecksumSHA256, csSHA256)
			}
		}
	} else {
		if p.Fill != nil && p.Size > 0 {
			data = make([]byte, p.Size)
			for i, _ := range data {
				data[i] = *p.Fill
			}
		} else {
			return nil, errors.Errorf("no suitable data source for %s", p.Name)
		}
	}
	return data, nil
}

func (p *FirmwarePart) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(firmwarePart(*p))
	if err != nil {
		return nil, err
	}
	if len(p.properties) == 0 {
		return b, nil
	}
	eb, err := json.Marshal(p.properties)
	if err != nil {
		return nil, err
	}
	eb[0] = ','
	rb := append(b[:len(b)-1], eb...)
	return rb, nil
}

func isJSONField(i interface{}, k string) bool {
	t := reflect.Indirect(reflect.ValueOf(i)).Type()
	for fi := 0; fi < t.NumField(); fi++ {
		sf := t.Field(fi)
		jk := strings.Split(sf.Tag.Get("json"), ",")[0]
		if k == jk {
			return true
		}
	}
	return false
}

func (p *FirmwarePart) UnmarshalJSON(b []byte) error {
	// Start by filling in the struct fields.
	var fp firmwarePart
	if err := json.Unmarshal(b, &fp); err != nil {
		return err
	}
	*p = FirmwarePart(fp)
	// Re-parse as a generic map.
	var mp map[string]interface{}
	json.Unmarshal(b, &mp)
	// Find keys that are not fields in the struct and add them as properties.
	for k, v := range mp {
		if !isJSONField(p, k) {
			if p.properties == nil {
				p.properties = make(map[string]interface{})
			}
			p.properties[k] = v
		}
	}
	return nil
}
