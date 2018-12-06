package fs

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"cesanta.com/common/go/lptr"
	"cesanta.com/common/go/ourutil"
	fwfs "cesanta.com/fw/defs/fs"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/flags"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

var (
	longFormat = flag.BoolP("long", "l", false, "Long output format.")
)

func listFiles(ctx context.Context, devConn *dev.DevConn, path string) (res []fwfs.ListExtResult, err error) {
	if *longFormat {
		res, err = devConn.CFilesystem.ListExt(ctx, &fwfs.ListExtArgs{Path: &path})
	} else {
		// Get file list from the attached device
		names, err := devConn.CFilesystem.List(ctx, &fwfs.ListArgs{Path: &path})
		if err != nil {
			return nil, errors.Trace(err)
		}
		for _, name := range names {
			n := name
			res = append(res, fwfs.ListExtResult{Name: &n})
		}
	}
	return res, err
}

type byName []fwfs.ListExtResult

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return strings.Compare(*a[i].Name, *a[j].Name) < 0 }

func Ls(ctx context.Context, devConn *dev.DevConn) error {
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

func GetFile(ctx context.Context, devConn *dev.DevConn, name string) (string, error) {
	// Get file
	contents := []byte{}
	var offset int64

	attempts := *flags.FsOpAttempts
	for {
		// Get the next chunk of data
		ctx2, cancel := context.WithTimeout(ctx, devConn.GetTimeout())
		defer cancel()
		glog.V(1).Infof("Getting %s %d @ %d (attempts %d)", name, flags.ChunkSize, offset, attempts)
		chunk, err := devConn.CFilesystem.Get(ctx2, &fwfs.GetArgs{
			Filename: &name,
			Offset:   lptr.Int64(offset),
			Len:      lptr.Int64(int64(*flags.ChunkSize)),
		})
		if err != nil {
			attempts -= 1
			if attempts > 0 {
				glog.Warningf("Error: %s", err)
				continue
			}
			// TODO(dfrank): probably handle out of memory error by retrying with a
			// smaller chunk size
			return "", errors.Trace(err)
		}
		attempts = *flags.FsOpAttempts

		decoded, err := base64.StdEncoding.DecodeString(*chunk.Data)
		if err != nil {
			return "", errors.Trace(err)
		}

		contents = append(contents, decoded...)
		offset += int64(len(decoded))

		// Check if there is some data left
		if *chunk.Left == 0 {
			break
		}
	}
	return string(contents), nil
}

func Get(ctx context.Context, devConn *dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return errors.Errorf("filename is required")
	}
	if len(args) > 2 {
		return errors.Errorf("extra arguments")
	}
	filename := args[1]
	text, err := GetFile(ctx, devConn, filename)
	if err != nil {
		return errors.Trace(err)
	}
	fmt.Print(string(text))
	return nil
}

func Put(ctx context.Context, devConn *dev.DevConn) error {
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

func PutFile(ctx context.Context, devConn *dev.DevConn, hostFilename, devFilename string) error {
	fileData, err := ourutil.ReadOrFetchFile(hostFilename)
	if err != nil {
		return errors.Trace(err)
	}

	return PutData(ctx, devConn, bytes.NewBuffer(fileData), devFilename)
}

func PutData(ctx context.Context, devConn *dev.DevConn, r io.Reader, devFilename string) error {
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
				if _, err := dev.CallDeviceService(ctx2, devConn, "FS.Put", putArgs); err != nil {
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

func fsRemoveFile(ctx context.Context, devConn *dev.DevConn, devFilename string) error {
	return errors.Trace(devConn.CFilesystem.Remove(ctx, &fwfs.RemoveArgs{
		Filename: &devFilename,
	}))
}

func Rm(ctx context.Context, devConn *dev.DevConn) error {
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
