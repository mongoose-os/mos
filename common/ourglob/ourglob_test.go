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
package ourglob

import "testing"

type expect struct {
	matcher Matcher
	tests   []expectStr
}

type expectStr struct {
	s       string
	want    bool
	wantErr string
}

func TestGlobmatcher(t *testing.T) {
	vals := []expect{
		expect{
			matcher: PatItems{{"foo/bar", true}, {"foo/*", false}},
			tests: []expectStr{
				{"foo/a1", false, ""},
				{"foo/a1/a2", false, ""},
				{"foo/bar", true, ""},
				{"foo/bar/hey", true, ""},
				{"foo/a2", false, ""},
				{"hey", false, ""},
			},
		},

		expect{
			matcher: &Pat{
				Items: []Item{{"foo/bar", true}, {"foo/*", false}, {"*", true}},
			},
			tests: []expectStr{
				{"foo/a1", false, ""},
				{"foo/bar", true, ""},
				{"foo/a2", false, ""},
				{"hey", true, ""},
			},
		},
	}

	for _, v := range vals {
		for _, test := range v.tests {
			match, err := v.matcher.Match(test.s)

			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}

			if errMsg != test.wantErr {
				t.Fatalf("matcher: %v, string: %q, wantErr: %q, got: %q", v.matcher, test.s, test.wantErr, errMsg)
			}

			if match != test.want {
				t.Fatalf("matcher: %v, string: %q, want: %v, got: %v", v.matcher, test.s, test.want, match)
			}
		}
	}
}
