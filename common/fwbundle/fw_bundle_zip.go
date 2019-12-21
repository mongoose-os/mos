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
package fwbundle

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/json"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/juju/errors"
	zip "github.com/mongoose-os/mos/common/ourzip"
	"github.com/mongoose-os/mos/cli/ourutil"
)

const (
	ManifestFileName = "manifest.json"

	// The ZIP AppNOte 6.3.6 says:
	//   Header IDs of 0 thru 31 are reserved for use by PKWARE.
	//   The remaining IDs can be used by third party vendors for
	//   proprietary usage.
	// 0x293a looks like a smiley face when written in LE (0x3a 0x29), so... hey, why not? :)
	zipExtraDataID = uint16(0x293a)
)

func ReadZipFirmwareBundle(fname string) (*FirmwareBundle, error) {
	var r *zip.Reader

	zipData, err := ourutil.ReadOrFetchFile(fname)
	if err != nil {
		return nil, errors.Trace(err)
	}

	r, err = zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, errors.Annotatef(err, "%s: invalid firmware file", fname)
	}

	fwb := NewBundle()

	blobs := make(map[string][]byte)

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, errors.Annotatef(err, "%s: failed to open", fname)
		}
		data, err := ioutil.ReadAll(rc)
		if err != nil {
			return nil, errors.Annotatef(err, "%s: failed to read", fname)
		}
		rc.Close()
		blobs[path.Base(f.Name)] = data
	}
	manifestData := blobs[ManifestFileName]
	if manifestData == nil {
		return nil, errors.Errorf("%s: no %s in the archive", fname, ManifestFileName)
	}
	err = json.Unmarshal(manifestData, &fwb.FirmwareManifest)
	if err != nil {
		return nil, errors.Annotatef(err, "%s: failed to parse manifest", fname)
	}
	for n, p := range fwb.FirmwareManifest.Parts {
		p.Name = n
		p.SetDataProvider(func(name, src string) ([]byte, error) {
			data, ok := blobs[src]
			if !ok {
				return nil, errors.Errorf("%s not found in the archive", src)
			}
			return data, nil
		})
	}
	return fwb, nil
}

func WriteZipFirmwareBytes(fwb *FirmwareBundle, buf *bytes.Buffer, compress bool, extraAttrs map[string]interface{}) error {
	zw := zip.NewWriter(buf)
	// When compressing, use best compression.
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})
	// Rewrite sources to be relative to archive.
	for _, p := range fwb.Parts {
		if p.Src == "" {
			continue
		}
		data, err := p.GetData()
		if err != nil {
			return errors.Annotatef(err, "%s: failed to calculate checksum", p.Name)
		}
		p.SetData(data)
		p.Src = filepath.Base(p.Src)
		if err := p.CalcChecksum(); err != nil {
			return errors.Annotatef(err, "%s: failed to calculate checksum", p.Name)
		}
	}
	manifestData, err := json.MarshalIndent(&fwb.FirmwareManifest, "", " ")
	if err != nil {
		return errors.Annotatef(err, "error marshaling manifest")
	}
	extraData := bytes.NewBuffer(nil)
	if len(extraAttrs) > 0 {
		extraAttrData, err := json.Marshal(extraAttrs)
		if err != nil {
			return errors.Annotatef(err, "error marshaling extra attrs")
		}
		binary.Write(extraData, binary.LittleEndian, zipExtraDataID)
		binary.Write(extraData, binary.LittleEndian, uint16(len(extraAttrData)))
		extraData.Write(extraAttrData)
	}
	glog.V(1).Infof("Manifest:\n%s", string(manifestData))
	zfh := &zip.FileHeader{
		Name:  ManifestFileName,
		Extra: extraData.Bytes(),
	}
	if compress {
		zfh.Method = zip.Deflate
	} else {
		zfh.Method = zip.Store
	}
	if err := zw.AddFile(zfh, manifestData); err != nil {
		return errors.Annotatef(err, "error adding %s", ManifestFileName)
	}
	for _, p := range fwb.Parts {
		if p.Src == "" {
			continue
		}
		data, err := p.GetData()
		if err != nil {
			return errors.Annotatef(err, "error getting data for %s", p.Name)
		}
		zfh = &zip.FileHeader{Name: p.Src}
		if compress {
			zfh.Method = zip.Deflate
		} else {
			zfh.Method = zip.Store
		}
		if err := zw.AddFile(zfh, data); err != nil {
			return errors.Annotatef(err, "%s: error adding %s", p.Name, p.Src)
		}
	}
	if err = zw.Close(); err != nil {
		return errors.Annotatef(err, "error closing the archive")
	}
	return nil
}

func WriteZipFirmwareBundle(fwb *FirmwareBundle, fname string, compress bool, extraAttrs map[string]interface{}) error {
	buf := new(bytes.Buffer)
	if err := WriteZipFirmwareBytes(fwb, buf, compress, extraAttrs); err != nil {
		return err
	}
	return ioutil.WriteFile(fname, buf.Bytes(), 0644)
}
