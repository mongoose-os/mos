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
package archive

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/juju/errors"
)

// UnzipInto unpacks a zipped stream into a directory skipping skipLevels top level directories
func UnzipInto(input io.ReaderAt, size int64, dir string, skipLevels int) error {
	zipReader, err := zip.NewReader(input, size)
	if err != nil {
		return errors.Trace(err)
	}

	for _, f := range zipReader.File {
		if err := unzipFileInto(f, dir, skipLevels); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func unzipFileInto(file *zip.File, dir string, skipLevels int) error {
	r, err := file.Open()
	if err != nil {
		return errors.Trace(err)
	}
	defer r.Close()

	// zip files have always forward slashes
	cs := strings.Split(file.Name, "/")
	if len(cs) <= skipLevels {
		return errors.Errorf(
			"path %q contains fewer elements than what skip levels (%d)",
			file.Name, skipLevels)
	}

	filePath := filepath.Join(dir, filepath.Join(cs[skipLevels:]...))
	fileInfo := file.FileInfo()

	if fileInfo.IsDir() {
		if err := os.MkdirAll(filePath, fileInfo.Mode()); err != nil {
			return errors.Trace(err)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return errors.Trace(err)
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			l, err := ioutil.ReadAll(r)
			if err != nil {
				return errors.Trace(err)
			}
			return errors.Trace(os.Symlink(string(l), filePath))
		}

		dest, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileInfo.Mode())
		if err != nil {
			return errors.Trace(err)
		}
		defer dest.Close()

		if _, err := io.Copy(dest, r); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}
