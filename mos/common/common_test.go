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
