/*
 * Copyright (c) 2014-2018 Cesanta Software Limited
 * All rights reserved
 *
 * Licensed under the Apache License, Version 2.0 (the ""License"");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an ""AS IS"" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ourutil

import (
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// GetPathForDocker replaces OS-dependent separators in a given path with "/"
func GetPathForDocker(p string) string {
	isWindows := (runtime.GOOS == "windows")
	ret := path.Join(strings.Split(p, string(filepath.Separator))...)
	if filepath.IsAbs(p) || (isWindows && strings.HasPrefix(p, string(filepath.Separator))) {
		if isWindows && ret[1] == ':' {
			// Remove the colon after drive letter, also lowercase the drive letter
			// (the lowercasing part is important for docker toolbox: there, host
			// paths like C:\foo\bar don't work, this path becomse /c/foo/bar)
			ret = fmt.Sprint(strings.ToLower(ret[:1]), ret[2:])
		}
		ret = path.Join("/", ret)
	}
	return ret
}
