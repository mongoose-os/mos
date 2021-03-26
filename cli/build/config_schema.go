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
	"bytes"
	"encoding/json"

	"github.com/juju/errors"
)

// ConfigSchemaItem represents a single config schema item, like this:
//
//     ["foo.bar", "default value"]
//
// or this:
//
//     ["foo.bar", "o", {"title": "Some title"}]
//
// Unfortunately we can't just use []interface{}, because
// {"title": "Some title"} gets unmarshaled as map[interface{}]interface{},
// which is an invalid type for JSON, so we have to create a custom type which
// implements json.Marshaler interface.
type ConfigSchemaItem []interface{}

func (c ConfigSchemaItem) MarshalJSON() ([]byte, error) {
	var data bytes.Buffer

	if _, err := data.WriteString("["); err != nil {
		return nil, errors.Trace(err)
	}

	for idx, v := range c {

		if idx > 0 {
			if _, err := data.WriteString(","); err != nil {
				return nil, errors.Trace(err)
			}
		}

		switch v2 := v.(type) {
		case string, bool, float64, int:
			// Primitives are marshaled as is
			d, err := json.Marshal(v2)
			if err != nil {
				return nil, errors.Trace(err)
			}

			data.Write(d)

		case map[interface{}]interface{}:
			// map[interface{}]interface{} needs to be converted to
			// map[string]interface{} before marshaling
			vjson := map[string]interface{}{}

			for k, v := range v2 {
				kstr, ok := k.(string)
				if !ok {
					return nil, errors.Errorf("invalid key: %v (must be a string)", k)
				}

				vjson[kstr] = v
			}

			d, err := json.Marshal(vjson)
			if err != nil {
				return nil, errors.Trace(err)
			}

			data.Write(d)

		default:
			return nil, errors.Errorf("invalid schema value: %v (type: %T)", v, v)
		}
	}

	if _, err := data.WriteString("]"); err != nil {
		return nil, errors.Trace(err)
	}

	return data.Bytes(), nil
}
