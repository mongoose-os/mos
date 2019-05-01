package ota

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/flags"
	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
)

var (
	commitTimeoutFlag = flag.Duration("commit-timeout", 300*time.Second,
		"If set, update must be explicitly committed within this time after finishing")
	updateTimeoutFlag = flag.Duration("update-timeout", 600*time.Second,
		"Timeout for entire update operation")
)

func OTA(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()
	fwFilename := ""
	beginArgs := ""
	switch len(args) {
	case 1:
		return errors.Errorf("firmware file is required")
	case 2:
		fwFilename = args[1]
	case 3:
		fwFilename = args[1]
		beginArgs = args[2]
	default:
		return errors.Errorf("extra arguments")
	}

	fwFileData, err := ourutil.ReadOrFetchFile(fwFilename)
	if err != nil {
		return errors.Trace(err)
	}
	fwFileSize := len(fwFileData)

	ourutil.Reportf("Getting current OTA status...")
	st := struct {
		State int `json:"state"`
	}{State: -1}
	if err := devConn.Call(ctx, "OTA.Status", nil, &st); err != nil {
		return errors.Annotatef(err, "unable to get current OTA status")
	}
	if st.State != 0 && st.State != 2 /* MGOS_OTA_STATE_ERROR */ {
		return errors.Errorf("update is already in progress (%d), call OTA.End", st.State)
	}

	if beginArgs == "" {
		ba := struct {
			Timeout       int64 `json:"timeout"`
			CommitTimeout int64 `json:"commit_timeout"`
			Size          int64 `json:"size"`
		}{
			Timeout:       int64(*updateTimeoutFlag) / 1000000000,
			CommitTimeout: int64(*commitTimeoutFlag) / 1000000000,
			Size:          int64(fwFileSize),
		}
		baJSON, _ := json.Marshal(&ba)
		beginArgs = string(baJSON)
	}
	ourutil.Reportf("Starting an update (args: %s)...", beginArgs)
	if err = devConn.Call(ctx, "OTA.Begin", beginArgs, nil); err != nil {
		return errors.Annotatef(err, "unable to start an update")
	}

	ourutil.Reportf("Writing data...")
	fwFile := bytes.NewBuffer(fwFileData)
	data := make([]byte, *flags.ChunkSize)
	total := int64(0)
	lastReport := time.Now()
	for {
		n, err := fwFile.Read(data)
		if n < 0 {
			return errors.Annotatef(err, "error reading file data")
		} else if n == 0 {
			break
		}
		dataB64 := base64.StdEncoding.EncodeToString(data[:n])
		sta := struct {
			Offset int64  `json:"offset"`
			Data   string `json:"data"`
		}{Offset: total, Data: dataB64}
		for i := 0; i < 3; i++ {
			ctx2, _ := context.WithTimeout(ctx, devConn.GetTimeout())
			if err = devConn.Call(ctx2, "OTA.Write", &sta, nil); err == nil {
				break
			}
			if i == 2 {
				devConn.Call(ctx, "OTA.End", nil, nil)
				return errors.Annotatef(err, "write failed at offset %d", total)
			}
		}
		total += int64(n)
		if total%65536 == 0 || time.Since(lastReport) > 5*time.Second {
			ourutil.Reportf("  %d of %d (%.2f%%)", total, fwFileSize, float64(total)*100.0/float64(fwFileSize))
			lastReport = time.Now()
		}
	}

	ourutil.Reportf("Finalizing update...")
	return devConn.Call(ctx, "OTA.End", nil, nil)
}
