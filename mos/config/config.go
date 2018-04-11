package config

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cesanta.com/common/go/lptr"
	"cesanta.com/common/go/ourutil"
	fwconfig "cesanta.com/fw/defs/config"
	"cesanta.com/mos/dev"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

var (
	noSave   bool
	noReboot bool

	saveTimeout  = 10 * time.Second
	saveAttempts = 3
)

// register advanced flash specific commands
func init() {
	flag.BoolVar(&noReboot, "no-reboot", false,
		"Save config but don't reboot the device.")
	flag.BoolVar(&noSave, "no-save", false,
		"Don't save config and don't reboot the device")
}

func Get(ctx context.Context, devConn *dev.DevConn) error {
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
	devConf, err := devConn.GetConfig(ctx)
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

func Set(ctx context.Context, devConn *dev.DevConn) error {
	return SetWithArgs(ctx, devConn, flag.Args()[1:])
}

func SetWithArgs(
	ctx context.Context, devConn *dev.DevConn, args []string,
) error {
	if len(args) < 1 {
		return errors.Errorf("at least one path.to.value=value pair should be given")
	}

	// Get all config from the attached device
	ourutil.Reportf("Getting configuration...")
	devConf, err := devConn.GetConfig(ctx)
	if err != nil {
		return errors.Trace(err)
	}

	paramValues, err := parseParamValues(args)
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

func SetAndSave(ctx context.Context, devConn *dev.DevConn, devConf *dev.DevConf) error {
	// save changed conf
	ourutil.Reportf("Setting new configuration...")
	err := devConn.SetConfig(ctx, devConf)
	if err != nil {
		return errors.Trace(err)
	}

	attempts := saveAttempts
	for !noSave {
		if noReboot {
			ourutil.Reportf("Saving...")
		} else {
			ourutil.Reportf("Saving and rebooting...")
		}
		ctx2, cancel := context.WithTimeout(ctx, saveTimeout)
		defer cancel()
		err = devConn.CConf.Save(ctx2, &fwconfig.SaveArgs{
			Reboot: lptr.Bool(!noReboot),
		})
		if err != nil {
			attempts -= 1
			if attempts > 0 {
				glog.Warningf("Error: %s", err)
				continue
			}
			return errors.Trace(err)
		}

		if !noReboot {
			time.Sleep(200 * time.Millisecond)
		}
		break
	}

	return nil
}

func parseParamValues(args []string) (map[string]string, error) {
	ret := map[string]string{}
	for _, a := range args {
		// Split arg into two substring by "=" (so, param name name cannot contain
		// "=", but value can)
		subs := strings.SplitN(a, "=", 2)
		if len(subs) < 2 {
			return nil, errors.Errorf("missing value for %q", a)
		}
		ret[subs[0]] = subs[1]
	}
	return ret, nil
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
