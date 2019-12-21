package mdash

import (
	"context"
	"fmt"

	flag "github.com/spf13/pflag"

	"github.com/mongoose-os/mos/cli/config"
	"github.com/mongoose-os/mos/cli/dev"
)

func MdashSetup(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return fmt.Errorf("Usage: mos mdash-setup DEVICE_TOKEN")
	}
	devConf, err := dev.GetConfig(ctx, devConn)
	if err != nil {
		return fmt.Errorf("failed to connect to get device config: %v", err)
	}
	devConf.Set("dash.enable", "true")
	devConf.Set("dash.token", args[1])

	return config.SetAndSave(ctx, devConn, devConf)
}
