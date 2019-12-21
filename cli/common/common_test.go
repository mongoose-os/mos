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
package moscommon

import (
	"testing"
)

func TestExpandPlaceholders(t *testing.T) {
	for i, c := range []struct {
		s, ps, ss, res string
	}{
		{"", "", "", ""},
		{"abc", "def", "123", "abc"},
		{"a?c", "?", "123", "a3c"},
		{"a???c", "?", "123", "a123c"},
		{"a?????c", "?", "123", "a23123c"},
		{"a?X?X?c", "?X", "123", "a23123c"},
		{"a?X?X?c", "?X", "X", "aXXXXXc"},
		{"foo_??????.key", "?X", "0123456789AB", "foo_6789AB.key"},
	} {
		res := ExpandPlaceholders(c.s, c.ps, c.ss)
		if res != c.res {
			t.Errorf("%d %q %q %q: %q != %q", i, c.s, c.ps, c.ss, res, c.res)
		}
	}
}
