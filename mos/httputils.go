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
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/golang/glog"
	"github.com/juju/errors"
)

const (
	beOK APIStatus = -1 * iota
	beError
	beInvalidParametersError
	beDBError
	beInvalidLoginOrPasswordError
	beAccessDeniedError
	beLimitExceededError
	beInvaildEndpointError
	beNotFoundError
	beAlreadyExistsError
)

// APIStatus is the API status/error code, as returned in the SimpleAPIResponse
type APIStatus int

// SimpleAPIResponse is what most API calls return.
type SimpleAPIResponse struct {
	Status       APIStatus `json:"status"`
	ErrorMessage string    `json:"error_message"`
}

func callAPI(method string, args, res interface{}) error {
	ab, err := json.Marshal(args)
	if err != nil {
		return errors.Trace(err)
	}

	server, err := serverURL()
	if err != nil {
		return errors.Trace(err)
	}

	uri := fmt.Sprintf("%s/api/%s", server, method)
	glog.Infof("calling %q with: %s", uri, string(ab))
	req, err := http.NewRequest("POST", uri, bytes.NewReader(ab))
	if err != nil {
		return errors.Trace(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(*user, *pass)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Trace(err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return errors.Trace(err)
	}

	return nil
}

func serverURL() (*url.URL, error) {
	u, err := url.Parse(*server)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// If given URL does not contain scheme, assume http
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	return u, nil
}
