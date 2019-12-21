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
package config

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/golang/glog"
	"github.com/juju/errors"
	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/ourutil"
	flag "github.com/spf13/pflag"
)

var (
	saveTimeout  = 10 * time.Second
	saveAttempts = 3
)

func Get(ctx context.Context, devConn dev.DevConn) error {
	path := ""

	args := flag.Args()[1:]

	if len(args) > 1 {
		return errors.Errorf("only one path to value is allowed")
	}

	// If path is given, use it; otherwise, an empty string will be assumed,
	// which means "all config"
	if len(args) == 1 {
		path = args[0]
	}

	// Get all config from the attached device
	devConf, err := dev.GetConfigLevel(ctx, devConn, *flags.Level)
	if err != nil {
		return errors.Trace(err)
	}

	// Try to get requested value
	val, err := devConf.Get(path)
	if err != nil {
		return errors.Trace(err)
	}

	fmt.Println(val)

	return nil
}

func Set(ctx context.Context, devConn dev.DevConn) error {
	return SetWithArgs(ctx, devConn, flag.Args()[1:])
}

func SetWithArgs(
	ctx context.Context, devConn dev.DevConn, args []string,
) error {
	if len(args) < 1 {
		return errors.Errorf("at least one path.to.value=value pair should be given")
	}

	// Get all config from the attached device
	ourutil.Reportf("Getting configuration...")
	devConf, err := dev.GetConfigLevel(ctx, devConn, *flags.Level)
	if err != nil {
		return errors.Trace(err)
	}

	paramValues, err := moscommon.ParseParamValues(args)
	if err != nil {
		return errors.Trace(err)
	}

	// Try to set all provided values
	for path, val := range paramValues {
		err := devConf.Set(path, val)
		if err != nil {
			return errors.Trace(err)
		}
	}

	return SetAndSave(ctx, devConn, devConf)
}

func SetAndSaveLevel(ctx context.Context, devConn dev.DevConn, devConf *dev.DevConf, level int) error {
	// save changed conf
	arg := &dev.ConfigSetArg{
		Save:    !*flags.NoSave,
		Reboot:  !*flags.NoReboot,
		TryOnce: *flags.TryOnce,
	}
	if level > 0 {
		arg.Level = level
		ourutil.Reportf("Setting new configuration (level %d)...", arg.Level)
	} else {
		ourutil.Reportf("Setting new configuration...")
	}
	saved, err := dev.SetConfig(ctx, devConn, devConf, arg)
	if err != nil {
		return errors.Trace(err)
	}

	// Newer firmware (2.12+) doesn't need explicit save.
	if arg.Save && saved {
		if arg.TryOnce {
			ourutil.Reportf("Note: --try-once is set, config is valid for one reboot only")
		}
		if arg.Reboot {
			time.Sleep(200 * time.Millisecond)
		}
		return nil
	}

	attempts := saveAttempts
	for arg.Save {
		if !arg.Reboot {
			ourutil.Reportf("Saving...")
		} else {
			ourutil.Reportf("Saving and rebooting...")
		}
		ctx2, cancel := context.WithTimeout(ctx, saveTimeout)
		defer cancel()
		if err = devConn.Call(ctx2, "Config.Save", map[string]interface{}{
			"reboot":   arg.Reboot,
			"try_once": arg.TryOnce,
		}, nil); err != nil {
			attempts -= 1
			if attempts > 0 {
				glog.Warningf("Error: %s", err)
				continue
			}
			return errors.Trace(err)
		}
		if arg.TryOnce {
			ourutil.Reportf("Note: --try-once is set, config is valid for one reboot only")
		}

		if arg.Reboot {
			time.Sleep(200 * time.Millisecond)
		}
		break
	}

	return nil
}

func SetAndSave(ctx context.Context, devConn dev.DevConn, devConf *dev.DevConf) error {
	return SetAndSaveLevel(ctx, devConn, devConf, *flags.Level)
}

func ApplyDiff(devConf *dev.DevConf, newConf map[string]string) error {
	ourutil.Reportf("\nUpdating config:")
	keys := []string{}
	for k, _ := range newConf {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := devConf.Set(k, newConf[k]); err != nil {
			return errors.Annotatef(err, "failed to set %s", k)
		}
		ourutil.Reportf("  %s = %s", k, newConf[k])
	}
	return nil
}
