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
package ourfilepath

import (
	"path/filepath"
	"strings"
)

// GetFirstPathComponent returns first component of the given path. If given
// an empty string, it's returned back.
func GetFirstPathComponent(p string) string {
	parts := strings.Split(p, string(filepath.Separator))
	if len(parts) > 0 {
		return parts[0]
	}

	return ""
}
