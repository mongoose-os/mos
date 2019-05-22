package mdash

import (
	"context"
	"fmt"

	flag "github.com/spf13/pflag"

	"github.com/mongoose-os/mos/mos/config"
	"github.com/mongoose-os/mos/mos/dev"
)

func MdashSetup(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()
	if len(args) < 3 {
		return fmt.Errorf("Usage: mos mdash-setup DEVICE_ID DEVICE_PASSWORD")
	}
	devConf, err := dev.GetConfig(ctx, devConn)
	if err != nil {
		return fmt.Errorf("failed to connect to get device config: %v", err)
	}
	devConf.Set("device.id", args[1])
	devConf.Set("mqtt.user", args[1])
	devConf.Set("mqtt.pass", args[2])
	devConf.Set("mqtt.server", "mdash.net:8883")
	devConf.Set("mqtt.enable", "true")
	devConf.Set("mqtt.ssl_ca_cert", "ca.pem")

	return config.SetAndSave(ctx, devConn, devConf)
}
