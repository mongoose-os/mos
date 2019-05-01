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

package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cesanta/errors"
)

// BuildCtxInfo contains metadata about the build context: all source files
// which were uploaded to the fwbuilder, their sizes and hashes.
type BuildCtxInfo struct {
	Files BuildCtxInfoDir `json:"files"`
}

type BuildCtxInfoDir map[string]*BuildCtxInfoFile

type BuildCtxInfoFile struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir,omitempty"`

	// Relevant if IsDir is false
	Size int    `json:"size,omitempty"`
	Hash string `json:"hash,omitempty"`
}

// GetBuildCtxInfo treats src as *clean* uploaded sources (meaning, src should
// contain only uploaded sources, and nothing else), and calculates metadata
func GetBuildCtxInfo(src string) (*BuildCtxInfo, error) {
	bctxInfo := BuildCtxInfo{
		Files: BuildCtxInfoDir(map[string]*BuildCtxInfoFile{}),
	}

	if err := addBuildCtxInfoDir(&bctxInfo, src, src); err != nil {
		return nil, errors.Trace(err)
	}

	return &bctxInfo, nil
}

func addBuildCtxInfoDir(bctxInfo *BuildCtxInfo, src, cut string) error {
	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return errors.Trace(err)
	}

	for _, entry := range entries {
		var err error
		curPath := filepath.Join(src, entry.Name())
		bctxInfo.Files[curPath[len(cut)+1:]], err = getBuildCtxInfoFile(curPath, entry)
		if err != nil {
			return errors.Trace(err)
		}

		if entry.IsDir() {
			if err := addBuildCtxInfoDir(bctxInfo, curPath, cut); err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}

func getBuildCtxInfoFile(src string, entry os.FileInfo) (*BuildCtxInfoFile, error) {
	cur := BuildCtxInfoFile{
		Name:  entry.Name(),
		IsDir: entry.IsDir(),
	}

	if !entry.IsDir() {
		var err error
		cur.Size = int(entry.Size())
		cur.Hash, err = getFileHash(src)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	return &cur, nil
}

func getFileHash(src string) (string, error) {
	h := md5.New()

	f, err := os.Open(src)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return "", errors.Trace(err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
