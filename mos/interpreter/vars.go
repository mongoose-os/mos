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
package interpreter

import (
	"fmt"
	"regexp"

	"github.com/juju/errors"
)

var (
	// Note: we opted to use ${foo} instead of {{foo}}, because {{foo}} needs to
	// be quoted in yaml, whereas ${foo} does not.
	varRegexp = regexp.MustCompile(`\$\{[^}]+\}`)
)

func ExpandVars(interp *MosInterpreter, s string, skipFailed bool) (string, error) {
	var errRet error
	result := varRegexp.ReplaceAllStringFunc(s, func(v string) string {
		expr := v[2 : len(v)-1]
		val, err := interp.EvaluateExprString(expr)
		if err != nil {
			if skipFailed {
				return v
			}
			errRet = errors.Annotatef(err, "expanding expressions in %q", s)
		}
		return val
	})
	return result, errRet
}

func ExpandVarsSlice(interp *MosInterpreter, slice []string, skipFailed bool) ([]string, error) {
	ret := []string{}
	for _, s := range slice {
		s, err := ExpandVars(interp, s, skipFailed)
		if err != nil {
			return nil, errors.Trace(err)
		}
		ret = append(ret, s)
	}
	return ret, nil
}

func WrapMosExpr(s string) string {
	return fmt.Sprintf("${%s}", s)
}
