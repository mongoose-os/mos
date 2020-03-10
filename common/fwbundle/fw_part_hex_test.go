//
// Copyright (c) 2014-2019 Cesanta Software Limited
// All rights reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package fwbundle

import (
	"bytes"
	"testing"
)

type pTestCase struct {
	addr uint32
	data string
}

func TestParseHexBundle(t *testing.T) {
	cases := []struct {
		data  string
		fail  bool
		start uint32
		parts []*pTestCase
	}{
		// 0
		{data: "", fail: true},
		// 1
		{data: `
:040000004F484149DB
:00000001FF
`,
			start: 0,
			parts: []*pTestCase{
				&pTestCase{addr: 0, data: "OHAI"},
			},
		},
		// 2 - linear address
		{data: `
:020000040800F2
:040000004F484149DB
:00000001FF
`,
			start: 0,
			parts: []*pTestCase{
				&pTestCase{addr: 0x8000000, data: "OHAI"},
			},
		},
		// 3 - segment address, linear start address
		{data: `
:020000021000EC
:040000004F484149DB
:04000005000123458E
:00000001FF
`,
			start: 0x12345,
			parts: []*pTestCase{
				&pTestCase{addr: 0x10000, data: "OHAI"},
			},
		},
		// 4 - part continuations
		{data: `
:100000004F4D474F4D474F4D474F4D474F4D472160
:020000020001FB
:10000000575446575446575446575446575446211A
:10001000575446575446575446575446575446210A
:020000020003F9
:030000002121219A
:00000001FF
`,
			start: 0,
			parts: []*pTestCase{
				&pTestCase{addr: 0, data: "OMGOMGOMGOMGOMG!WTFWTFWTFWTFWTF!WTFWTFWTFWTFWTF!!!!"},
			},
		},
		// 5 - separate parts
		{data: `
:100000004F4D474F4D474F4D474F4D474F4D472160
:020000020001FB
:10000000575446575446575446575446575446211A
:10001000575446575446575446575446575446210A
:020000020300F9
:030000002121219A
:00000001FF
`,
			start: 0,
			parts: []*pTestCase{
				&pTestCase{addr: 0, data: "OMGOMGOMGOMGOMG!WTFWTFWTFWTFWTF!WTFWTFWTFWTFWTF!"},
				&pTestCase{addr: 0x3000, data: "!!!"},
			},
		},
	}

	for i, c := range cases {
		hb, err := ParseHexBundle([]byte(c.data), 255, 0)
		if c.fail {
			if err == nil {
				t.Fatalf("%d: %s: expected failure, got %#v", i, c.data, hb)
			}
		} else {
			if err != nil {
				t.Fatalf("%d: got error: %s", i, err)
			}
			if hb.Start != c.start {
				t.Fatalf("%d: invalid start address: expected 0x%x, got 0x%x", i, c.start, hb.Start)
			}
			if len(hb.Parts) != len(c.parts) {
				t.Fatalf("%d: invalid number of parts: expected %d, got %d", i, len(c.parts), len(hb.Parts))
			}
			for pi, cp := range c.parts {
				p := hb.Parts[pi]
				if p.Addr != cp.addr {
					t.Fatalf("%d: %d: invalid address: expected 0x%x, got 0x%x", i, pi, cp.addr, p.Addr)
				}
				if bytes.Compare(p.Data, []byte(cp.data)) != 0 {
					t.Fatalf("%d: %d: invalid data: expected %q, got %q", i, pi, cp.data, string(p.Data))
				}
			}
		}
	}
}
