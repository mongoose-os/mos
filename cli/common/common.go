// Copyright (c) 2014-2017 Cesanta Software Limited
// All rights reserved

package moscommon

import (
	"strconv"
	"strings"

	"github.com/juju/errors"
)

const (
	BuildTargetDefault = "all"
)

// ExpandPlaceholders expands placeholders in s with characters from ss, starting from the right.
// Placeholders are specified by ps.
func ExpandPlaceholders(s, ps, ss string) string {
	res := ""
	ssi := len(ss) - 1
	for si := len(s) - 1; si >= 0; si-- {
		c := s[si]
		if strings.IndexByte(ps, c) >= 0 {
			c = ss[ssi]
			ssi--
			if ssi < 0 {
				ssi = len(ss) - 1
			}
		}
		res = string(c) + res
	}
	return res
}

func ParseParamValues(args []string) (map[string]string, error) {
	ret := map[string]string{}
	for _, a := range args {
		// Split arg into two substring by "=" (so, param name name cannot contain
		// "=", but value can)
		subs := strings.SplitN(a, "=", 2)
		if len(subs) < 2 {
			return nil, errors.Errorf("missing value for %q", a)
		}
		ret[subs[0]] = subs[1]
	}
	return ret, nil
}

func ParseParamValuesTyped(args []string) (map[string]interface{}, error) {
	var res map[string]interface{}
	params, err := ParseParamValues(args)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for p, valueStr := range params {
		var value interface{}
		switch valueStr {
		case "true":
			value = true
		case "false":
			value = false
		default:
			if i, err := strconv.ParseInt(valueStr, 0, 64); err == nil {
				value = i
			} else {
				value = valueStr
			}
		}
		if res == nil {
			res = make(map[string]interface{})
		}
		res[p] = value
	}
	return res, nil
}
