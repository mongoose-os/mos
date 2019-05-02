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

import "strings"

// GetVersionSuffix returns suffix like "-1.5" or "-latest". See
// GetVersionSuffixTpl.
func GetVersionSuffix(version string) string {
	return GetVersionSuffixTpl(version, "-${version}")
}

// GetVersionSuffixTpl returns given template with "${version}" placeholder
// replaced with the actual given version. If given version is "master" or
// an empty string, "latest" is used instead.
func GetVersionSuffixTpl(version, template string) string {
	if version == "master" || version == "" {
		version = "latest"
	}
	return strings.Replace(template, "${version}", version, -1)
}
