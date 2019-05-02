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
	"bytes"
	"fmt"
)

// Error bundles multiple errors and make them obey the error interface
type Error struct {
	errs []error
}

func (e *Error) Error() string {
	buf := bytes.NewBuffer(nil)

	fmt.Fprintf(buf, "%d error(s) occurred:", len(e.errs))
	for _, err := range e.errs {
		fmt.Fprintf(buf, "\n%s", err)
	}
	return buf.String()
}

// Append creates a new mutlierror.Error structure or appends the arguments to an existing multierror
// err can be nil, or can be a non-multierror error.
func Append(err error, errs ...error) error {
	if err == nil {
		return &Error{errs}
	}
	switch err := err.(type) {
	case *Error:
		err.errs = append(err.errs, errs...)
		return err
	default:
		return &Error{append([]error{err}, errs...)}
	}
}
