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
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/juju/errors"
)

var mdLinkRegex = regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)

func fetchGitLabAsset(host, repoPath, tag, assetName, token string) ([]byte, error) {
	apiURLPrefix := fmt.Sprintf("https://%s/api/v4/projects/%s", host, url.QueryEscape(repoPath))
	relMetaURL := fmt.Sprintf("%s/releases/%s", apiURLPrefix, tag)
	client := &http.Client{}
	req, err := http.NewRequest("GET", relMetaURL, nil)
	if token != "" {
		req.Header.Add("PRIVATE-TOKEN", token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to fetch %s", relMetaURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("got %d status code when fetching %s (note: private repos may need --gh-token)", resp.StatusCode, relMetaURL)
	}
	relMetaData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Trace(err)
	}
	glog.V(4).Infof("%s/%s/%s: Release metadata: %s", repoPath, tag, assetName, string(relMetaData))

	var relMeta struct {
		Name        string `json:"name"`
		TagName     string `json:"tag_name"`
		Description string `json:"description"`
		Assets      struct {
			Links []*struct {
				Name     string `json:"name"`
				URL      string `json:"url"`
				LinkType string `json:"link_type"`
			} `json:"assets"`
		}
	}
	if err = json.Unmarshal(relMetaData, &relMeta); err != nil {
		return nil, errors.Annotatef(err, "failed to parse GitLab release info")
	}

	// GitLab releases are kind of a mess. There is no way to attach binary assets directly,
	// (https://gitlab.com/gitlab-org/gitlab/-/issues/17838).
	// But it's possible to attack "link assets" and it's possible to upload files.
	// It's not very convenient to do from the UI, so we also parse description to look for links.

	// 1. Check link assets. Name must match and type must be "package".
	assetURL := ""
	for _, a := range relMeta.Assets.Links {
		if a.Name == assetName && a.LinkType == "package" {
			assetURL = a.URL
			break
		}
	}

	// 2. Parse markdown description to look for link with the specific title.
	if assetURL == "" {
		for _, m := range mdLinkRegex.FindAllStringSubmatch(relMeta.Description, -1) {
			linkName, linkURL := m[1], m[2]
			if linkName == assetName {
				assetURL = linkURL
				break
			}
		}
	}

	if assetURL == "" {
		return nil, errors.Annotatef(os.ErrNotExist, "%s: no asset %s found in release %s", repoPath, assetName, tag)
	}

	if strings.HasPrefix(assetURL, "/") {
		assetURL = fmt.Sprintf("https://%s/%s%s", host, repoPath, assetURL)
	}

	glog.Infof("%s/%s/%s: Asset URL: %s", repoPath, tag, assetName, assetURL)

	// Probably won't work due to https://gitlab.com/gitlab-org/gitlab/-/issues/24155
	// but hey, give it a go.
	return fetchAssetFromURL(host, assetName, tag, assetURL, token)
}
