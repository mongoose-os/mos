// Copyright (c) 2014-2017 Cesanta Software Limited
// All rights reserved

package moscommon

import (
	"strings"
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
