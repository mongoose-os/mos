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
	"fmt"
	"testing"

	"github.com/juju/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartFromString(t *testing.T) {
	ff := byte(0xff)
	cases := []struct {
		s    string
		fail bool
		p    *FirmwarePart
	}{
		{s: ``, fail: true},
		{s: `foo:`, p: &FirmwarePart{Name: "foo"}},
		{s: `foo:-`, p: nil},
		{s: `foo:type`, fail: true},
		{s: `foo:type=bar`, p: &FirmwarePart{Name: "foo", Type: "bar"}},
		{s: `foo:type="bar"`, p: &FirmwarePart{Name: "foo", Type: "bar"}},
		{s: `foo:type='bar'`, p: &FirmwarePart{Name: "foo", Type: "bar"}},
		{s: `foo:type=123`, fail: true},
		{s: `foo:fill=0xff`, p: &FirmwarePart{Name: "foo", Fill: &ff}},
		{s: `foo:type=bar,encrypt=false`, p: &FirmwarePart{Name: "foo", Type: "bar"}},
		{s: `foo:type=bar,encrypt=true`, p: &FirmwarePart{Name: "foo", Type: "bar", ESP32Encrypt: true}},
		{s: `app:addr=0x100000,src=/bar/baz.bin`,
			p: &FirmwarePart{Name: "app", Addr: 0x100000, Src: "/bar/baz.bin"}},
		{s: `boot:addr=0x0,src=/boot.bin,update=false`,
			p: &FirmwarePart{Name: "boot",
				Src:   "/boot.bin",
				attrs: map[string]interface{}{"update": false},
			}},
	}
	for _, c := range cases {
		pn, p, err := PartFromString(c.s)
		if c.fail {
			assert.Errorf(t, err, "case %s, ret %+v", c.s, p)
			assert.Nilf(t, p, "case %s", c.s)
		} else {
			require.NoError(t, err, "case %s, ret %+v", c.s, p)
			assert.Equalf(t, c.p, p, "case %s", c.s)
			if c.p != nil {
				assert.Equalf(t, p.Name, pn, "case %s", c.s)
			}
		}
	}
}

func TestGetData(t *testing.T) {
	p := &FirmwarePart{Name: "foo", Src: "foo.bin"}
	if _, err := p.GetData(); err == nil {
		t.Fatalf("expected to fail")
	}
	p.SetData([]byte("bar"))
	data, err := p.GetData()
	if err != nil {
		t.Fatalf("got error %s", err)
	}
	if string(data) != "bar" {
		t.Fatalf("got %q", data)
	}
	if p.ChecksumSHA1 != "62cdb7020ff920e5aa642c3d4066950dd1f01f4d" {
		t.Fatalf("unexpected sha1 %s", p.ChecksumSHA1)
	}
	if p.ChecksumSHA256 != "fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9" {
		t.Fatalf("unexpected sha256 %s", p.ChecksumSHA256)
	}
	p.ChecksumSHA1 = "72cdb7020ff920e5aa642c3d4066950dd1f01f4d"
	if _, err := p.GetData(); err == nil {
		t.Fatalf("expected to fail checksum check")
	}
	p.ChecksumSHA1 = "62cdb7020ff920e5aa642c3d4066950dd1f01f4d"
	p.ChecksumSHA256 = "ecde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9"
	if _, err := p.GetData(); err == nil {
		t.Fatalf("expected to fail checksum check")
	}
	p.ChecksumSHA1 = ""
	if err != nil {
		t.Fatalf("got error %s", err)
	}
	if string(data) != "bar" {
		t.Fatalf("got %q", data)
	}
}

func TestGetDataFill(t *testing.T) {
	a := byte(0x61)
	p := &FirmwarePart{Name: "foo", Fill: &a}
	if _, err := p.GetData(); err == nil {
		t.Fatalf("expected to fail")
	}
	p.Size = 10
	data, err := p.GetData()
	if err != nil {
		t.Fatalf("got error %s", err)
	}
	if string(data) != "aaaaaaaaaa" {
		t.Fatalf("got %q", data)
	}
}

func TestGetDataFromProvider(t *testing.T) {
	blobs := map[string]string{
		"foo:foo.bin": "bar",
	}
	p := &FirmwarePart{Name: "foo", Src: "foo.bin", ChecksumSHA1: "62cdb7020ff920e5aa642c3d4066950dd1f01f4d"}
	p.SetDataProvider(func(name, src string) ([]byte, error) {
		data, ok := blobs[fmt.Sprintf("%s:%s", name, src)]
		if !ok {
			return nil, errors.Errorf("not found")
		}
		return []byte(data), nil
	})
	data, err := p.GetData()
	if err != nil {
		t.Fatalf("got error %s", err)
	}
	if string(data) != "bar" {
		t.Fatalf("got %q", data)
	}
	if p.ChecksumSHA1 != "62cdb7020ff920e5aa642c3d4066950dd1f01f4d" {
		t.Fatalf("unexpected sha1 %s", p.ChecksumSHA1)
	}
	p.ChecksumSHA1 = "72cdb7020ff920e5aa642c3d4066950dd1f01f4d"
	if _, err := p.GetData(); err == nil {
		t.Fatalf("expected to fail checksum check")
	}
	p.ChecksumSHA1 = ""
	if err != nil {
		t.Fatalf("got error %s", err)
	}
	if string(data) != "bar" {
		t.Fatalf("got %q", data)
	}
}
