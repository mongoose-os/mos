package devutil

import (
	"context"
	"crypto/tls"
	"runtime"
	"strings"
	"time"

	"cesanta.com/common/go/mgrpc/codec"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/flags"
	"cesanta.com/mos/watson"
	"github.com/cesanta/errors"
)

func createDevConnWithJunkHandler(ctx context.Context, junkHandler func(junk []byte)) (dev.DevConn, error) {
	port, err := GetPort()
	if err != nil {
		return nil, errors.Trace(err)
	}
	c := dev.Client{Port: port, Timeout: *flags.Timeout, Reconnect: *flags.Reconnect}
	prefix := "serial://"
	if strings.Index(port, "://") > 0 {
		prefix = ""
	}
	addr := prefix + port

	// Init and pass TLS config if --cert-file and --key-file are specified
	var tlsConfig *tls.Config
	if *flags.CertFile != "" ||
		strings.HasPrefix(port, "wss") ||
		strings.HasPrefix(port, "https") ||
		strings.HasPrefix(port, "mqtts") {

		tlsConfig, err = flags.TLSConfigFromFlags()
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	codecOpts := &codec.Options{
		AzureDM: codec.AzureDMCodecOptions{
			ConnectionString: *flags.AzureConnectionString,
		},
		GCP: codec.GCPCodecOptions{
			CreateTopic: *flags.GCPRPCCreateTopic,
		},
		MQTT: codec.MQTTCodecOptions{},
		Serial: codec.SerialCodecOptions{
			BaudRate:             uint(*flags.BaudRate),
			HardwareFlowControl:  *flags.HWFC,
			JunkHandler:          junkHandler,
			SetControlLines:      *flags.SetControlLines,
			InvertedControlLines: *flags.InvertedControlLines,
		},
		Watson: codec.WatsonCodecOptions{
			APIKey:       watson.WatsonAPIKeyFlag,
			APIAuthToken: watson.WatsonAPIAuthTokenFlag,
		},
	}
	// Due to lack of flow control, we send data in chunks and wait after each.
	// At non-default baud rate we assume user knows what they are doing.
	if !codecOpts.Serial.HardwareFlowControl &&
		codecOpts.Serial.BaudRate == 115200 &&
		!*flags.RPCUARTNoDelay {
		codecOpts.Serial.SendChunkSize = 16
		codecOpts.Serial.SendChunkDelay = 5 * time.Millisecond
		// So, this is weird. ST-Link serial device seems to have issues on Mac OS X if we write too fast.
		// Yes, even 16 bytes at 5 ms interval is too fast, and 8 bytes too. It looks like this:
		// processes trying to access /dev/cu.usbmodemX get stuck and the only re-plugging
		// (or re-enumerating) gets it out of this state.
		// Hence, the following kludge.
		if runtime.GOOS == "darwin" && strings.Contains(port, "cu.usbmodem") {
			codecOpts.Serial.SendChunkSize = 6
		}
	}
	devConn, err := c.CreateDevConnWithOpts(ctx, addr, *flags.Reconnect, tlsConfig, codecOpts)
	return devConn, errors.Trace(err)
}

func CreateDevConnFromFlags(ctx context.Context) (dev.DevConn, error) {
	return createDevConnWithJunkHandler(ctx, func(junk []byte) {})
}
