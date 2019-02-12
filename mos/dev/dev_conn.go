package dev

import (
	"context"
	"time"

	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

type ConfigSetArg struct {
	Config map[string]interface{} `json:"config,omitempty"`
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

func SetConfig(ctx context.Context, dc DevConn, devConf *DevConf) error {
	attempts := confOpAttempts
	for {
		ctx2, cancel := context.WithTimeout(ctx, dc.GetTimeout())
		defer cancel()
		if err := dc.Call(ctx2, "Config.Set", &ConfigSetArg{
			Config: devConf.diff,
		}, nil); err != nil {
			attempts -= 1
			if attempts > 0 {
				glog.Warningf("Error: %s", err)
				continue
			}
			return errors.Trace(err)
		}
		break
	}

	return nil
}

func GetConfig(ctx context.Context, dc DevConn) (*DevConf, error) {
	var devConf DevConf
	attempts := confOpAttempts
	for {
		ctx2, cancel := context.WithTimeout(ctx, dc.GetTimeout())
		defer cancel()
		if err := dc.Call(ctx2, "Config.Get", nil, &devConf.data); err != nil {
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
