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
package pflagenv

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
)

func TestParseFlagSet(t *testing.T) {
	fs := pflag.NewFlagSet("pflagenv-test", pflag.ContinueOnError)

	var myFlag1, myFlag2, myFlag3, myFlag4 string
	fs.StringVar(&myFlag1, "my-flag1", "def1", "")
	fs.StringVar(&myFlag2, "my-flag2", "def2", "")
	fs.StringVar(&myFlag3, "my-flag3", "def3", "")
	fs.StringVar(&myFlag4, "my-flag4", "def4", "")
	fs.Parse([]string{"--my-flag1=cl1", "--my-flag2="})

	os.Setenv("TEST_MY_FLAG1", "env1")
	os.Setenv("TEST_MY_FLAG2", "env2")
	os.Setenv("TEST_MY_FLAG3", "env3")
	ParseFlagSet(fs, "TEST_")

	if got, want := myFlag1, "cl1"; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}

	if got, want := myFlag2, ""; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}

	if got, want := myFlag3, "env3"; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}

	if got, want := myFlag4, "def4"; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}
