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
package build

import (
	"testing"
)

func TestParseGitLocation(t *testing.T) {
	cases := []struct {
		loc                       string
		fail                      bool
		rh, rp, rn, ln, url, path string
	}{
		{loc: "", fail: true},
		{loc: "foo", fail: true},
		{loc: "ssh://example.org/home/foo/bar",
			rh: "example.org", rp: "home/foo/bar", rn: "bar", ln: "bar", url: "ssh://example.org/home/foo/bar", path: ""},
		{loc: "ssh://example.org/home/foo/bar.git",
			rh: "example.org", rp: "home/foo/bar", rn: "bar", ln: "bar", url: "ssh://example.org/home/foo/bar.git", path: ""},
		{loc: "ssh://example.org/home/foo/bar/baz",
			rh: "example.org", rp: "home/foo/bar/baz", rn: "baz", ln: "baz", url: "ssh://example.org/home/foo/bar/baz", path: ""},
		{loc: "ssh://example.org/home/foo/bar.git/baz",
			rh: "example.org", rp: "home/foo/bar", rn: "bar", ln: "baz", url: "ssh://example.org/home/foo/bar.git", path: "baz"},
		{loc: "ssh://example.org/home/foo/bar.git/baz/boo",
			rh: "example.org", rp: "home/foo/bar", rn: "bar", ln: "boo", url: "ssh://example.org/home/foo/bar.git", path: "baz/boo"},
		{loc: "example.org:foo.git",
			rh: "example.org", rp: "foo", rn: "foo", ln: "foo", url: "example.org:foo.git", path: ""},
		{loc: "example.org:foo.git/bar",
			rh: "example.org", rp: "foo", rn: "foo", ln: "bar", url: "example.org:foo.git", path: "bar"},
		{loc: "example.org:foo/bar.git",
			rh: "example.org", rp: "foo/bar", rn: "bar", ln: "bar", url: "example.org:foo/bar.git", path: ""},
		{loc: "git@github.com:foo/bar.git",
			rh: "github.com", rp: "foo/bar", rn: "bar", ln: "bar", url: "git@github.com:foo/bar.git", path: ""},
		{loc: "git@example.org:foo/bar.git",
			rh: "example.org", rp: "foo/bar", rn: "bar", ln: "bar", url: "git@example.org:foo/bar.git", path: ""},
		{loc: "example.org:foo/bar.git/baz",
			rh: "example.org", rp: "foo/bar", rn: "bar", ln: "baz", url: "example.org:foo/bar.git", path: "baz"},
		{loc: "git@github.com:foo/bar.git/baz",
			rh: "github.com", rp: "foo/bar", rn: "bar", ln: "baz", url: "git@github.com:foo/bar.git", path: "baz"},
		{loc: "git@example.org:foo/bar.git/baz",
			rh: "example.org", rp: "foo/bar", rn: "bar", ln: "baz", url: "git@example.org:foo/bar.git", path: "baz"},
		{loc: "git@example.org:foo/bar.git/baz/boo",
			rh: "example.org", rp: "foo/bar", rn: "bar", ln: "boo", url: "git@example.org:foo/bar.git", path: "baz/boo"},
		{loc: "https://github.com/foo/bar",
			rh: "github.com", rp: "foo/bar", rn: "bar", ln: "bar", url: "https://github.com/foo/bar", path: ""},
		{loc: "https://github.com/foo/bar/tree/master/baz",
			rh: "github.com", rp: "foo/bar", rn: "bar", ln: "baz", url: "https://github.com/foo/bar", path: "baz"},
		{loc: "https://github.com/foo/bar/tree/master/baz/boo",
			rh: "github.com", rp: "foo/bar", rn: "bar", ln: "boo", url: "https://github.com/foo/bar", path: "baz/boo"},
		{loc: "https://gitlab.example.org/foo/-/tree/master/bar",
			rh: "gitlab.example.org", rp: "foo", rn: "foo", ln: "bar", url: "https://gitlab.example.org/foo", path: "bar"},
		{loc: "https://gitlab.example.org/foo/bar/-/tree/master/baz",
			rh: "gitlab.example.org", rp: "foo/bar", rn: "bar", ln: "baz", url: "https://gitlab.example.org/foo/bar", path: "baz"},
		{loc: "https://gitlab.example.org/foo/-/tree/master/bar/baz",
			rh: "gitlab.example.org", rp: "foo", rn: "foo", ln: "baz", url: "https://gitlab.example.org/foo", path: "bar/baz"},
	}

	for _, c := range cases {
		repoHost, repoPath, repoName, libName, repoURL, pathWithinRepo, err := parseGitLocation(c.loc)
		if !c.fail {
			if err != nil {
				t.Errorf("%q: expected %q %q %q %q %q, got error instead (%s)", c.loc, c.rp, c.rn, c.ln, c.url, c.path, err)
			} else if repoHost != c.rh || repoPath != c.rp || repoName != c.rn || libName != c.ln || repoURL != c.url || pathWithinRepo != c.path {
				t.Errorf("%q: expected %q %q %q %q %q %q, got %q %q %q %q %q %q instead",
					c.loc, c.rh, c.rp, c.rn, c.ln, c.url, c.path,
					repoHost, repoPath, repoName, libName, repoURL, pathWithinRepo)
			}
		} else if err == nil {
			t.Errorf("%q: expected an error, got %q %q %q %q instead", c.loc, repoName, libName, repoURL, pathWithinRepo)
		}
	}
}
