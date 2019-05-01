package fwbundle

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
)

func TestFirmwareManifestEncoding(t *testing.T) {
	mjs := `
{
  "build_id": "20181126-182907/2.8.0-47-g7db8cdacf-dirty-mos9",
  "build_timestamp": "2018-11-26T18:29:07Z",
  "name": "ccm",
  "parts": {
    "app": {
      "addr": 65536,
      "cs_sha1": "3fe36826a8c4fa628903ee897182b78e8e2b9590",
      "encrypt": true,
      "ptn": "app_0",
      "size": 1375120,
      "src": "ccm.bin",
      "type": "app"
    },
    "boot": {
      "addr": 4096,
      "cs_sha1": "d24bf3566e6437baed946f8f6b08481ec730766e",
      "encrypt": true,
      "size": 21088,
      "src": "bootloader.bin",
      "type": "boot",
      "update": false
    }
  },
  "platform": "esp32",
  "version": "1.0"
}
`
	var m FirmwareManifest
	var mj1, mj2 interface{}

	if err := json.Unmarshal([]byte(mjs), &m); err != nil {
		t.Errorf("err %v", err)
		return
	}
	if err := json.Unmarshal([]byte(mjs), &mj1); err != nil {
		t.Errorf("err %v", err)
		return
	}
	mb, err := json.MarshalIndent(&m, "", "  ")
	if err != nil {
		t.Errorf("err %v", err)
		return
	}
	if err := json.Unmarshal(mb, &mj2); err != nil {
		t.Errorf("err %v", err)
		return
	}
	if !reflect.DeepEqual(mj1, mj2) {
		mjs1, _ := json.MarshalIndent(mj1, "", "  ")
		mjs2, _ := json.MarshalIndent(mj2, "", "  ")
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(mjs1), string(mjs2), false)
		t.Errorf("manifest encoding incorrect:\n%s\n\n %s", string(mb), dmp.DiffPrettyText(diffs))
		return
	}
}
