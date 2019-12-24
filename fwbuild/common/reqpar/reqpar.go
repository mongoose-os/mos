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

package reqpar

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/juju/errors"
)

const (
	// Needed for ParseMultipartForm; this is a default value which is used when
	// we call FormValue and friends. For some reason it's unexported, so I
	// had to just paste it here.
	defaultMaxMemory = 32 << 20 // 32 MB
)

// RequestParams represents simplified HTTP form data: string values
// and files.
type RequestParams struct {
	Values map[string][]string      `json:"values"`
	Files  map[string][]RequestFile `json:"files"`
}

type RequestFile struct {
	// Name of the temporary file in which contents are stored
	Filename string `json:"filename"`
	// Original filename used by the client
	OrigFilename string `json:"orig_filename"`
}

// New creates JSON-able RequestParams structure from the given
// http request. All form files are stored as temporary files, so the caller is
// responsible for cleaning those up, see RemoveFiles.
func New(r *http.Request, tmpDir string, payloadLimit int64) (ret *RequestParams, err error) {
	r.ParseMultipartForm(defaultMaxMemory)

	if r.MultipartForm == nil {
		return nil, errors.New("not a multipart request")
	}

	par := &RequestParams{
		Values: map[string][]string{},
		Files:  map[string][]RequestFile{},
	}

	defer func() {
		// In case of an error, cleanup files which have been created so far
		if err != nil {
			par.RemoveFiles()
		}
	}()

	// Handle string request values
	for k, v := range r.MultipartForm.Value {
		par.Values[k] = v
	}

	// Handle request files: for each of them, create a temporary file
	for k, v := range r.MultipartForm.File {
		files := []RequestFile{}
		for _, v2 := range v {
			reqFile, err := v2.Open()
			if err != nil {
				return nil, errors.Trace(err)
			}
			defer func() {
				reqFile.Close()
			}()

			curFile, err := ioutil.TempFile(tmpDir, "input_file_")
			if err != nil {
				return nil, errors.Trace(err)
			}
			defer func() {
				curFile.Close()
			}()

			written, err := io.Copy(curFile, reqFile)
			if err != nil {
				return nil, errors.Trace(err)
			}

			if written > payloadLimit {
				return nil, errors.Errorf("max size %d exceeded (upload size is %d)", payloadLimit, written)
			}

			files = append(files, RequestFile{
				OrigFilename: v2.Filename,
				Filename:     curFile.Name(),
			})
		}
		par.Files[k] = files
	}

	return par, nil
}

// FormFileName returns name of the temporary file
func (rp *RequestParams) FormFileName(name string) string {
	files := rp.Files[name]
	if len(files) > 0 {
		return files[0].Filename
	}
	return ""
}

func (rp *RequestParams) FormValue(name string) string {
	vals := rp.Values[name]
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// RemoveFiles removes all files saved as part of request params
func (rp *RequestParams) RemoveFiles() {
	for _, v := range rp.Files {
		for _, vf := range v {
			os.RemoveAll(vf.Filename)
		}
	}
}
