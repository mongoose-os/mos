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
package fs

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/ourutil"
	flag "github.com/spf13/pflag"
)

var (
	longFormat = flag.BoolP("long", "l", false, "Long output format.")
)

type ListArgs struct {
	Path *string `json:"path,omitempty"`
}

type ListExtArgs struct {
	Path *string `json:"path,omitempty"`
}

type ListExtResult struct {
	Name *string `json:"name,omitempty"`
	Size *int64  `json:"size,omitempty"`
}

func listFiles(ctx context.Context, devConn dev.DevConn, path string) ([]ListExtResult, error) {
	var res []ListExtResult
	if *longFormat {
		if err := devConn.Call(ctx, "FS.ListExt", &ListExtArgs{Path: &path}, &res); err != nil {
			return nil, errors.Trace(err)
		}
	} else {
		// Get file list from the attached device
		var names []string
		if err := devConn.Call(ctx, "FS.List", &ListArgs{Path: &path}, &names); err != nil {
			return nil, errors.Trace(err)
		}
		for _, name := range names {
			n := name
			res = append(res, ListExtResult{Name: &n})
		}
	}
	return res, nil
}

type byName []ListExtResult

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return strings.Compare(*a[i].Name, *a[j].Name) < 0 }

func Ls(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()
	path := "/"
	if len(args) >= 2 {
		path = args[1]
	}
	files, err := listFiles(ctx, devConn, path)
	if err != nil {
		return errors.Trace(err)
	}
	sort.Sort(byName(files))
	for _, file := range files {
		fmt.Printf("%s", *file.Name)
		if file.Size != nil {
			fmt.Printf(" %d", *file.Size)
		}
		fmt.Printf("\r\n")
	}
	return nil
}

type GetArgs struct {
	Filename *string `json:"filename,omitempty"`
	Len      int64   `json:"len"`
	Offset   int64   `json:"offset"`
}

type GetResult struct {
	Data *string `json:"data,omitempty"`
	Left *int64  `json:"left,omitempty"`
}

func getFileSink(ctx context.Context, devConn dev.DevConn, name string, sink io.Writer) error {
	// Get file
	var offset int64

	attempts := *flags.FsOpAttempts
	for {
		// Get the next chunk of data
		ctx2, cancel := context.WithTimeout(ctx, devConn.GetTimeout())
		defer cancel()
		glog.V(1).Infof("Getting %s %d @ %d (attempts %d)", name, flags.ChunkSize, offset, attempts)
		var chunk GetResult
		if err := devConn.Call(ctx2, "FS.Get", &GetArgs{
			Filename: &name,
			Offset:   offset,
			Len:      int64(*flags.ChunkSize),
		}, &chunk); err != nil {
			attempts -= 1
			if attempts > 0 {
				glog.Warningf("Error: %s", err)
				continue
			}
			// TODO(dfrank): probably handle out of memory error by retrying with a
			// smaller chunk size
			return errors.Trace(err)
		}
		attempts = *flags.FsOpAttempts

		decoded, err := base64.StdEncoding.DecodeString(*chunk.Data)
		if err != nil {
			return errors.Trace(err)
		}

		offset += int64(len(decoded))

		for len(decoded) > 0 {
			if n, err := sink.Write(decoded); err != nil {
				return errors.Trace(err)
			} else {
				decoded = decoded[n:]
			}
		}

		// Check if there is some data left
		if *chunk.Left == 0 {
			break
		}
	}
	return nil
}

func GetFile(ctx context.Context, devConn dev.DevConn, name string) (string, error) {
	buf := bytes.NewBuffer(nil)
	err := getFileSink(ctx, devConn, name, buf)
	return string(buf.Bytes()), err
}

func Get(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return errors.Errorf("filename is required")
	}
	if len(args) > 2 {
		return errors.Errorf("extra arguments")
	}
	filename := args[1]
	return errors.Trace(getFileSink(ctx, devConn, filename, os.Stdout))
}

func Put(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return errors.Errorf("filename is required")
	}
	if len(args) > 3 {
		return errors.Errorf("extra arguments")
	}
	hostFilename := args[1]
	devFilename := path.Base(hostFilename)

	// If device filename was given, use it.
	if len(args) >= 3 {
		devFilename = args[2]
	}

	return PutFile(ctx, devConn, hostFilename, devFilename)
}

func PutFile(ctx context.Context, devConn dev.DevConn, hostFilename, devFilename string) error {
	fileData, err := ourutil.ReadOrFetchFile(hostFilename)
	if err != nil {
		return errors.Trace(err)
	}

	return PutData(ctx, devConn, bytes.NewBuffer(fileData), devFilename)
}

func PutData(ctx context.Context, devConn dev.DevConn, r io.Reader, devFilename string) error {
	data := make([]byte, *flags.ChunkSize)
	appendFlag := false

	offset := 0
	attempts := *flags.FsOpAttempts
	for {
		// Read the next chunk from the file.
		n, readErr := r.Read(data)
		if n > 0 {
			for attempts > 0 {
				ctx2, cancel := context.WithTimeout(ctx, devConn.GetTimeout())
				defer cancel()
				glog.V(1).Infof("Sending %s %d @ %d (attempts %d)", devFilename, n, offset, attempts)
				putArgs := &struct {
					Filename string `json:"filename"`
					Offset   int    `json:"offset"`
					Append   bool   `json:"append"`
					Data     string `json:"data"`
				}{
					Filename: devFilename,
					Offset:   offset,
					Append:   appendFlag,
					Data:     base64.StdEncoding.EncodeToString(data[:n]),
				}
				if err := devConn.Call(ctx2, "FS.Put", putArgs, nil); err != nil {
					attempts -= 1
					if attempts > 0 {
						glog.Warningf("Error: %s", err)
						continue
					}
					return errors.Trace(err)
				} else {
					break
				}
			}
		}
		attempts = *flags.FsOpAttempts
		if readErr != nil {
			if errors.Cause(readErr) == io.EOF {
				// Reached EOF, quit the loop normally.
				break
			}
			// Some non-EOF error, return error.
			return errors.Trace(readErr)
		}

		// All subsequent writes to this file will append the chunk.
		appendFlag = true
		offset += n
	}

	return nil
}

type RemoveArgs struct {
	Filename *string `json:"filename,omitempty"`
}

func fsRemoveFile(ctx context.Context, devConn dev.DevConn, devFilename string) error {
	return errors.Trace(devConn.Call(ctx, "FS.Remove", &RemoveArgs{
		Filename: &devFilename,
	}, nil))
}

func Rm(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return errors.Errorf("filename is required")
	}
	if len(args) > 2 {
		return errors.Errorf("extra arguments")
	}
	filename := args[1]
	return errors.Trace(fsRemoveFile(ctx, devConn, filename))
}
