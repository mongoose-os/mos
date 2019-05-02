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
package ourglob

import (
	"path/filepath"
	"strings"

	"github.com/cesanta/errors"
)

type Matcher interface {
	Match(s string) (bool, error)
}

type Pat struct {
	// Patterns to check and corresponding match values. Patterns are checked
	// in order, corresponding Match value is returned at the first pattern
	// match. If no patterns matched, false will be returned.
	Items PatItems
}

type PatItems []Item

type Item struct {
	Pattern string
	Match   bool
}

func (m *Pat) Match(s string) (bool, error) {
	for _, item := range m.Items {
		patParts := strings.Split(item.Pattern, string(filepath.Separator))
		parts := strings.Split(s, string(filepath.Separator))

		// Unfortunately, filepath.Match can only match the whole string, not part
		// of it, and ** is not supported, so we have to just manually cut
		// string if it has more components than the pattern
		if len(parts) > len(patParts) {
			s = strings.Join(parts[:len(patParts)], string(filepath.Separator))
		}

		matched, err := filepath.Match(item.Pattern, s)
		if err != nil {
			return false, errors.Trace(err)
		}

		if matched {
			return item.Match, nil
		}
	}

	// No matching item; assuming no match
	return false, nil
}

// PatItems.Match is a shortcut for Pat.Match with the default options
func (items PatItems) Match(s string) (bool, error) {
	Pat := Pat{
		Items: items,
	}
	return Pat.Match(s)
}
