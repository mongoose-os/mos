//go:generate ../common/tools/fw_meta.py gen_build_info --go_output=./version.go  --json_output=./version.json
//go:generate go-bindata -pkg main -nocompress -modtime 1 -mode 420 data/
//go:generate go-bindata-assetfs -pkg main -nocompress -modtime 1 -mode 420 data/ web_root/...

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cesanta.com/cloud/common/ide"
	"cesanta.com/common/go/pflagenv"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

const (
	envPrefix = "MOS_"
)

// This section contains all "simple" flags, i.e. flags that our great leader loves and cares about.
// Each command can also register more flags but they should be hidden by default so the tool doesn't seem complex.
// Full help can be shown with --helpfull anyway.
var (
	arch       = flag.String("arch", "", "Hardware architecture. Possible values: esp8266, cc3200")
	user       = flag.String("user", "", "Cloud username")
	pass       = flag.String("pass", "", "Cloud password or token")
	server     = flag.String("server", "http://mongoose.cloud", "Cloud server")
	local      = flag.Bool("local", false, "Local build.")
	mosRepo    = flag.String("repo", "", "Path to the mongoose-os repository; if omitted, the mongoose-os repository will be cloned as ./mongoose-os")
	deviceID   = flag.String("device-id", "", "Device ID")
	devicePass = flag.String("device-pass", "", "Device pass/key")
	firmware   = flag.String("firmware", filepath.Join(buildDir, ide.FirmwareFileName), "Firmware .zip file location (file of HTTP URL)")
	port       = flag.String("port", "", "Serial port where the device is connected")
	timeout    = flag.Duration("timeout", 10*time.Second, "Timeout for the device connection")
	reconnect  = flag.Bool("reconnect", false, "Enable reconnection")
	force      = flag.Bool("force", false, "Use the force")
	verbose    = flag.Bool("verbose", false, "Verbose output")
	gui        = flag.Bool("ui", false, "Start GUI")

	versionFlag = flag.Bool("version", false, "Print version and exit")
	helpFull    = flag.Bool("helpfull", false, "Show full help, including advanced flags")

	extendedMode = false
)

var (
	// put all commands here
	commands = []command{
		{"init", initFW, `Initialise firmware directory structure in the current directory`, []string{}, []string{"arch", "force"}},
		{"build", build, `Build a firmware from the sources located in the current directory`, []string{}, []string{"arch", "local", "repo", "clean", "server"}},
		{"flash", flash, `Flash firmware to the device`, []string{"port"}, []string{"firmware"}},
		{"console", console, `Simple serial port console`, []string{"port"}, []string{}},
		{"ls", fsLs, `List files at the local device's filesystem`, []string{"port"}, []string{}},
		{"get", fsGet, `Read file from the local device's filesystem and print to stdout`, []string{"port"}, []string{}},
		{"put", fsPut, `Put file from the host machine to the local device's filesystem`, []string{"port"}, []string{}},
		{"config-get", configGet, `Get config value from the locally attached device`, []string{"port"}, []string{}},
		{"config-set", configSet, `Set config value at the locally attached device`, []string{"port"}, []string{}},
		{"call", call, `Perform a device API call. "mos call RPC.List" shows available methods`, []string{"port"}, []string{}},
		{"aws-iot-setup", awsIoTSetup, `Provision the device for AWS IoT cloud`, []string{"port"}, []string{"use-atca", "atca-slot", "aws-region"}},
	}
	// These commands are only available when invoked with -X
	extendedCommands = []command{
		{"atca-get-config", atcaGetConfig, `Get ATCA chip config`, []string{"port"}, []string{"format"}},
		{"atca-set-config", atcaSetConfig, `Set ATCA chip config`, []string{"port"}, []string{"format", "dry-run"}},
		{"atca-lock-zone", atcaLockZone, `Lock config or data zone`, []string{"port"}, []string{"dry-run"}},
		{"atca-set-key", atcaSetKey, `Set key in a given slot`, []string{"port"}, []string{"dry-run", "write-key"}},
		{"atca-gen-key", atcaGenKey, `Generate a random key in a given slot`, []string{"port"}, []string{"dry-run"}},
		{"atca-get-pub-key", atcaGetPubKey, `Retrieve public ECC key from a given slot`, []string{"port"}, []string{}},
		{"esp32-encrypt-image", esp32EncryptImage, `Encrypt a ESP32 firmware image`, []string{"esp32-encryption-key-file", "esp32-flash-address"}, []string{}},
	}
)

type command struct {
	name     string
	handler  handler
	short    string
	required []string
	optional []string
}

type handler func() error

func unimplemented() error {
	fmt.Println("TODO")
	return nil
}

func run() error {
	for _, c := range commands {
		if c.name == flag.Arg(0) {
			// check required flags
			if err := checkFlags(c.required); err != nil {
				return errors.Trace(err)
			}
			// run the handler
			if err := c.handler(); err != nil {
				return errors.Trace(err)
			}
			return nil
		}
	}
	// not found
	usage()
	return nil
}

func main() {
	// -X, if given, must be the first arg.
	if len(os.Args) > 1 && os.Args[1] == "-X" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		extendedMode = true
		commands = append(commands, extendedCommands...)
	}
	initFlags()
	flag.Parse()
	pflagenv.Parse(envPrefix)

	if *helpFull {
		unhideFlags()
		usage()
	} else if *versionFlag {
		fmt.Printf(
			"%s\nVersion: %s\nBuild ID: %s\n",
			"The Mongoose OS command line tool", Version, BuildId,
		)
		return
	}

	if *gui == true {
		startUI()
	}

	if err := run(); err != nil {
		glog.Infof("Error: %+v", err)
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
