package dev

import (
	"context"
	"time"

	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

type ConfigSetArg struct {
	Config map[string]interface{} `json:"config,omitempty"`
	// Since 2.12
	Level   int  `json:"level,omitempty"`
	Save    bool `json:"save,omitempty"`
	Reboot  bool `json:"reboot,omitempty"`
	TryOnce bool `json:"try_once,omitempty"`
}

type ConfigSetResp struct {
	Saved bool `json:"saved,omitempty"`
}

type GetInfoResultWifi struct {
	SSSID  *string `json:"ssid,omitempty"`
	StaIP  *string `json:"sta_ip,omitempty"`
	APIP   *string `json:"ap_ip,omitempty"`
	Status *string `json:"status,omitempty"`
}

type GetInfoResult struct {
	App        *string            `json:"app,omitempty"`
	Arch       *string            `json:"arch,omitempty"`
	Fs_free    *int64             `json:"fs_free,omitempty"`
	Fs_size    *int64             `json:"fs_size,omitempty"`
	Fw_id      *string            `json:"fw_id,omitempty"`
	Fw_version *string            `json:"fw_version,omitempty"`
	Mac        *string            `json:"mac,omitempty"`
	RAMFree    *int64             `json:"ram_free,omitempty"`
	RAMMinFree *int64             `json:"ram_min_free,omitempty"`
	RAMSize    *int64             `json:"ram_size,omitempty"`
	Uptime     *int64             `json:"uptime,omitempty"`
	Wifi       *GetInfoResultWifi `json:"wifi,omitempty"`
}

type DevConn interface {
	Call(ctx context.Context, method string, args interface{}, resp interface{}) error
	GetTimeout() time.Duration
	Connect(context.Context, bool) error
	Disconnect(context.Context) error
}

const confOpAttempts = 3

func SetConfig(ctx context.Context, dc DevConn, devConf *DevConf, setArgTmpl *ConfigSetArg) (bool, error) {
	var resp ConfigSetResp
	if setArgTmpl == nil {
		setArgTmpl = &ConfigSetArg{}
	}
	setArg := *setArgTmpl
	setArg.Config = devConf.diff
	attempts := confOpAttempts
	for {
		ctx2, cancel := context.WithTimeout(ctx, dc.GetTimeout())
		defer cancel()
		if err := dc.Call(ctx2, "Config.Set", setArg, &resp); err != nil {
			attempts -= 1
			if attempts > 0 {
				glog.Warningf("Error: %s", err)
				continue
			}
			return false, errors.Trace(err)
		}
		glog.Infof("resp: %#v", resp)
		break
	}

	return resp.Saved, nil
}

func GetConfigLevel(ctx context.Context, dc DevConn, level int) (*DevConf, error) {
	var devConf DevConf
	attempts := confOpAttempts
	for {
		ctx2, cancel := context.WithTimeout(ctx, dc.GetTimeout())
		defer cancel()
		var err error
		if level >= 0 {
			err = dc.Call(ctx2, "Config.Get", struct {
				Level int `json:"level"`
			}{Level: level}, &devConf.data)
		} else {
			err = dc.Call(ctx2, "Config.Get", nil, &devConf.data)
		}
		if err != nil {
			attempts -= 1
			if attempts > 0 {
				glog.Warningf("Error: %s", err)
				continue
			}
			return nil, errors.Trace(err)
		}
		break
	}
	return &devConf, nil
}

func GetConfig(ctx context.Context, dc DevConn) (*DevConf, error) {
	return GetConfigLevel(ctx, dc, -1)
}

func GetInfo(ctx context.Context, dc DevConn) (*GetInfoResult, error) {
	var r GetInfoResult
	attempts := confOpAttempts
	for {
		ctx2, cancel := context.WithTimeout(ctx, dc.GetTimeout())
		defer cancel()
		var err error
		if err = dc.Call(ctx2, "Sys.GetInfo", nil, &r); err == nil {
			return &r, nil
		}
		attempts -= 1
		if attempts > 0 {
			glog.Warningf("Error: %s", err)
			continue
		}
		return nil, errors.Trace(err)
	}
}
