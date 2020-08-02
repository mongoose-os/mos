//
// Copyright (c) 2014-2020 Cesanta Software Limited
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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/juju/errors"

	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/ourutil"
)

func fetchGitHubAsset(loc, host, repoPath, tag, assetName string) ([]byte, error) {
	var apiURLPrefix string
	if host == "github.com" {
		// Try public URL first. Most of our repos (and therefore assets) are public.
		// API access limits do not apply to public asset access.
		if strings.HasPrefix(loc, "https://") {
			assetURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoPath, tag, assetName)
			data, err := fetchAssetFromURL(host, assetName, tag, assetURL)
			if err == nil {
				return data, nil
			}
		}
		apiURLPrefix = "https://api.github.com"
	} else {
		apiURLPrefix = fmt.Sprintf("https://%s/api/v3", host)
	}
	relMetaURL := fmt.Sprintf("%s/repos/%s/releases/tags/%s", apiURLPrefix, repoPath, tag)
	client := &http.Client{}
	req, err := http.NewRequest("GET", relMetaURL, nil)
	token, err := getToken(*flags.GHToken, host)
	if err != nil {
		return nil, errors.Annotatef(err, "invalid --gh-token")
	}
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to fetch %s", relMetaURL)
	}
	defer resp.Body.Close()
	assetURL := ""
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("got %d status code when fetching %s (note: private repos may need --gh-token)", resp.StatusCode, relMetaURL)
	}
	relMetaData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Trace(err)
	}
	glog.V(4).Infof("%s/%s/%s: Release metadata: %s", repoPath, tag, assetName, string(relMetaData))
	var relMeta struct {
		ID     int `json:"id"`
		Assets []*struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"assets"`
	}
	if err = json.Unmarshal(relMetaData, &relMeta); err != nil {
		return nil, errors.Annotatef(err, "failed to parse GitHub release info")
	}
	for _, a := range relMeta.Assets {
		if a.Name == assetName {
			assetURL = a.URL
			break
		}
	}
	if assetURL == "" {
		return nil, errors.Annotatef(os.ErrNotExist, "%s: no asset %s found in release %s", repoPath, assetName, tag)
	}
	glog.Infof("%s/%s/%s: Asset URL: %s", repoPath, tag, assetName, assetURL)
	return fetchAssetFromURL(host, assetName, tag, assetURL)
}

func fetchAssetFromURL(host, assetName, tag, assetURL string) ([]byte, error) {
	ourutil.Reportf("Fetching %s (%s) from %s...", assetName, tag, assetURL)

	client := &http.Client{}
	req, err := http.NewRequest("GET", assetURL, nil)
	req.Header.Add("Accept", "application/octet-stream")
	token, err := getToken(*flags.GHToken, host)
	if err != nil {
		return nil, errors.Annotatef(err, "invalid --gh-token")
	}
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", token)) // GitHub
		req.Header.Add("PRIVATE-TOKEN", token)                          // GitLab
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to fetch %s", assetURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("got %d status code when fetching %s", resp.StatusCode, assetURL)
	}
	// Fetched the asset successfully
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return data, nil
}
