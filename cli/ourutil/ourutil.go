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
package ourutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/golang/glog"
)

func Reportf(f string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, f+"\n", args...)
	glog.Infof(f, args...)
}

func Freportf(logFile io.Writer, f string, args ...interface{}) {
	fmt.Fprintf(logFile, f+"\n", args...)
	glog.Infof(f, args...)
}

func Prompt(text string) string {
	fmt.Fprintf(os.Stderr, "%s ", text)
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(ans)
}

func FirstN(s string, n int) string {
	if n > len(s) {
		n = len(s)
	}
	return s[:n]
}

// Returns a map from regexp capture group name to the corresponding matched
// string.
// A return value of nil indicates no match.
func FindNamedSubmatches(r *regexp.Regexp, s string) map[string]string {
	matches := r.FindStringSubmatch(s)
	if matches == nil {
		return nil
	}

	result := make(map[string]string)
	for i, name := range r.SubexpNames()[1:] {
		result[name] = matches[i+1]
	}
	return result
}
