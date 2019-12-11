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
package multierror

import (
	"testing"

	"github.com/juju/errors"
)

func TestAppend(t *testing.T) {
	var err error
	err = Append(err, errors.Errorf("an error"))
	if err == nil {
		t.Fatal(err)
	}

	if got, want := err.Error(), `1 error(s) occurred:
an error`; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}

	err = Append(err, errors.Errorf("another error"))
	if got, want := err.Error(), `2 error(s) occurred:
an error
another error`; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}

	err = errors.Errorf("old error")
	err = Append(err, errors.Errorf("new error"))
	if err == nil {
		t.Fatal(err)
	}

	if got, want := err.Error(), `2 error(s) occurred:
old error
new error`; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}

}
