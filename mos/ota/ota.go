package ota

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"time"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/fs" // For ChunkSizeFlag
	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
)

var (
	commitTimeoutFlag = flag.Duration("commit-timeout", 300*time.Second,
		"If set, update must be explicitly committed within this time after finishing")
	updateTimeoutFlag = flag.Duration("update-timeout", 600*time.Second,
		"Timeout for entire update operation")
)

func OTA(ctx context.Context, devConn *dev.DevConn) error {
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

	fwFile, err := os.Open(fwFilename)
	if err != nil {
		return errors.Annotatef(err, "unable to open fw file")
	}
	defer fwFile.Close()
	fi, err := fwFile.Stat()
	if err != nil {
		return errors.Annotatef(err, "unable to stat fw file")
	}

	ourutil.Reportf("Getting current OTA status...")
	s, err := dev.CallDeviceService(ctx, devConn, "OTA.Status", "")
	if err != nil {
		return errors.Annotatef(err, "unable to get current OTA status")
	}
	st := struct {
		State int `json:"state"`
	}{State: -1}
	if err := json.Unmarshal([]byte(s), &st); err != nil {
		return errors.Annotatef(err, "invalid OTA.Status response")
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
			Size:          fi.Size(),
		}
		baJSON, _ := json.Marshal(&ba)
		beginArgs = string(baJSON)
	}
	ourutil.Reportf("Starting an update (args: %s)...", beginArgs)
	s, err = dev.CallDeviceService(ctx, devConn, "OTA.Begin", beginArgs)
	if err != nil {
		return errors.Annotatef(err, "unable to start an update")
	}

	data := make([]byte, *fs.ChunkSizeFlag)
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
			Data string `json:"data"`
		}{Data: dataB64}
		staJSON, _ := json.Marshal(&sta)
		s, err = dev.CallDeviceService(ctx, devConn, "OTA.Write", string(staJSON))
		total += int64(n)
		if total%65536 == 0 || time.Since(lastReport) > 5*time.Second {
			ourutil.Reportf("  %d of %d (%.2f%%)", total, fi.Size(), float64(total)*100.0/float64(fi.Size()))
			lastReport = time.Now()
		}
	}

	ourutil.Reportf("Finalizing update...")
	s, err = dev.CallDeviceService(ctx, devConn, "OTA.End", beginArgs)
	return err
}
