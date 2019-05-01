//go:generate go-bindata-assetfs -pkg main -nocompress -modtime 1 -mode 420 web_root/...

package main

import (
	cRand "crypto/rand"
	goflag "flag"
	"fmt"
	"log"
	"math/big"
	mRand "math/rand"
	"os"
	"strings"
	"time"

	"context"

	"cesanta.com/common/go/pflagenv"
	"cesanta.com/mos/aws"
	"cesanta.com/mos/azure"
	"cesanta.com/mos/clone"
	moscommon "cesanta.com/mos/common"
	"cesanta.com/mos/common/paths"
	"cesanta.com/mos/common/state"
	"cesanta.com/mos/config"
	"cesanta.com/mos/create_fw_bundle"
	"cesanta.com/mos/debug_core_dump"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/devutil"
	"cesanta.com/mos/fs"
	"cesanta.com/mos/gcp"
	license "cesanta.com/mos/license_cmd"
	"cesanta.com/mos/ota"
	"cesanta.com/mos/update"
	"cesanta.com/mos/version"
	"cesanta.com/mos/watson"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

const (
	envPrefix = "MOS_"
)

// This section contains all "simple" flags, i.e. flags that our great leader loves and cares about.
// Each command can also register more flags but they should be hidden by default so the tool doesn't seem complex.
// Full help can be shown with --full anyway.
var (
	user       = flag.String("user", "", "Cloud username")
	pass       = flag.String("pass", "", "Cloud password or token")
	server     = flag.String("server", "https://mongoose.cloud", "FWBuild server")
	local      = flag.Bool("local", false, "Local build.")
	mosRepo    = flag.String("repo", "", "Path to the mongoose-os repository; if omitted, the mongoose-os repository will be cloned as ./mongoose-os")
	deviceID   = flag.String("device-id", "", "Device ID")
	devicePass = flag.String("device-pass", "", "Device pass/key")
	dryRun     = flag.Bool("dry-run", true, "Do not apply changes, print what would be done")
	firmware   = flag.String("firmware", moscommon.GetFirmwareZipFilePath(moscommon.GetBuildDir("")), "Firmware .zip file location (file of HTTP URL)")
	force      = flag.Bool("force", false, "Use the force")
	verbose    = flag.Bool("verbose", false, "Verbose output")
	chdir      = flag.StringP("chdir", "C", "", "Change into this directory first")
	xFlag      = flag.BoolP("enable-extended", "X", false, "Deprecated. Enable extended commands")

	helpFull = flag.Bool("full", false, "Show full help, including advanced flags")

	isUI = false
)

var (
	// put all commands here
	commands []command
)

type command struct {
	name        string
	handler     handler
	short       string
	required    []string
	optional    []string
	needDevConn YesNoMaybe
	extended    bool
}

type YesNoMaybe float32

const (
	Yes   YesNoMaybe = 1.0
	Maybe            = 0.5
	No               = 0.0
)

type handler func(ctx context.Context, devConn dev.DevConn) error

// channel of "junk" messages, which go to the console
var consoleMsgs chan []byte

func unimplemented() error {
	fmt.Println("TODO")
	return nil
}

func init() {
	commands = []command{
		{"ui", startUI, `Start GUI`, nil, nil, No, false},
		{"build", buildHandler, `Build a firmware from the sources located in the current directory`, nil, []string{"arch", "platform", "local", "repo", "clean", "server"}, No, false},
		{"clone", clone.Clone, `Clone a repo`, nil, []string{}, No, false},
		{"flash", flash, `Flash firmware to the device`, nil, []string{"port", "firmware"}, Maybe, false},
		{"flash-read", flashRead, `Read a region of flash`, []string{"platform"}, []string{"port"}, No, false},
		{"console", console, `Simple serial port console`, nil, []string{"port"}, No, false}, //TODO: needDevConn
		{"ls", fs.Ls, `List files at the local device's filesystem`, nil, []string{"port"}, Yes, false},
		{"get", fs.Get, `Read file from the local device's filesystem and print to stdout`, nil, []string{"port"}, Yes, false},
		{"put", fs.Put, `Put file from the host machine to the local device's filesystem`, nil, []string{"port"}, Yes, false},
		{"rm", fs.Rm, `Delete a file from the device's filesystem`, nil, []string{"port"}, Yes, false},
		{"ota", ota.OTA, `Perform an OTA update on a device`, nil, []string{"port"}, Yes, false},
		{"config-get", config.Get, `Get config value from the locally attached device`, nil, []string{"port"}, Yes, false},
		{"config-set", config.Set, `Set config value at the locally attached device`, nil, []string{"port"}, Yes, false},
		{"call", call, `Perform a device API call. "mos call RPC.List" shows available methods`, nil, []string{"port"}, Yes, false},
		{"create-fw-bundle", create_fw_bundle.CreateFWBundle, `Create or modify a firmware ZIP bundle from disparate parts.`, nil, nil, No, false},
		{"debug-core-dump", debug_core_dump.DebugCoreDump, `Debug a core dump`, nil, nil, No, false},
		{"aws-iot-setup", aws.AWSIoTSetup, `Provision the device for AWS IoT cloud`, nil, []string{"atca-slot", "aws-region", "port", "use-atca"}, Yes, false},
		{"azure-iot-setup", azure.AzureIoTSetup, `Provision the device for Azure IoT Hub`, nil, []string{"atca-slot", "azure-auth-file", "port", "use-atca"}, Yes, false},
		{"gcp-iot-setup", gcp.GCPIoTSetup, `Provision the device for Google IoT Core`, nil, []string{"atca-slot", "gcp-region", "port", "use-atca", "registry"}, Yes, false},
		{"watson-iot-setup", watson.WatsonIoTSetup, `Provision the device for IBM Watson IoT Platform`, nil, []string{}, Yes, false},
		{"update", update.Update, `Self-update mos tool; optionally update channel can be given (e.g. "latest", "release", or some exact version)`, nil, nil, No, false},
		{"license", license.License, `License device`, nil, nil, Maybe, false},
		{"license-save-key", license.SaveKey, `Save license server key`, nil, nil, No, false},
		{"wifi", wifi, `Setup WiFi - shortcut to config-set wifi...`, nil, nil, Yes, false},
		{"help", showHelp, `Show help. Add --full to show advanced commands`, nil, nil, No, false},
		{"version", showVersion, `Show version`, nil, nil, No, false},

		// extended commands
		{"atca-get-config", atcaGetConfig, `Get ATCA chip config`, nil, []string{"format", "port"}, Yes, true},
		{"atca-set-config", atcaSetConfig, `Set ATCA chip config`, nil, []string{"format", "dry-run", "port"}, Yes, true},
		{"atca-lock-zone", atcaLockZone, `Lock config or data zone`, nil, []string{"dry-run", "port"}, Yes, true},
		{"atca-set-key", atcaSetKey, `Set key in a given slot`, nil, []string{"dry-run", "port", "write-key"}, Yes, true},
		{"atca-gen-key", atcaGenKey, `Generate a random key in a given slot`, nil, []string{"dry-run", "port"}, Yes, true},
		{"atca-get-pub-key", atcaGetPubKey, `Retrieve public ECC key from a given slot`, nil, []string{"port"}, Yes, true},
		{"atca-gen-csr", atcaGenCSR, `Generate a random key in a given slot and generate a certificate request file`, nil, []string{"port"}, Yes, true},
		{"atca-gen-cert", atcaGenCert, `Generate a random key in a given slot and issue a certificate`, nil, []string{"port"}, Yes, true},
		{"esp32-efuse-get", esp32EFuseGet, `Get ESP32 eFuses`, nil, nil, No, true},
		{"esp32-efuse-set", esp32EFuseSet, `Set ESP32 eFuses`, nil, nil, No, true},
		{"esp32-encrypt-image", esp32EncryptImage, `Encrypt a ESP32 firmware image`, []string{"esp32-encryption-key-file", "esp32-flash-address"}, nil, No, true},
		{"esp32-gen-key", esp32GenKey, `Generate and program an encryption key`, nil, nil, No, true},
		{"eval-manifest-expr", evalManifestExpr, `Evaluate the expression against the final manifest`, nil, nil, No, true},
		{"get-mos-repo-dir", getMosRepoDir, `Show mongoose-os repo absolute path`, nil, nil, No, true},
		{"ports", showPorts, `Show serial ports`, nil, nil, No, true},
	}
}

func showHelp(ctx context.Context, devConn dev.DevConn) error {
	unhideFlags()
	usage()
	return nil
}

func showVersion(ctx context.Context, devConn dev.DevConn) error {
	fmt.Printf(
		"%s\nVersion: %s\nBuild ID: %s\nUpdate channel: %s\n",
		"The Mongoose OS command line tool", version.Version, version.BuildId, update.GetUpdateChannel(),
	)
	return nil
}

func showPorts(ctx context.Context, devConn dev.DevConn) error {
	fmt.Printf("%s\n", strings.Join(devutil.EnumerateSerialPorts(), "\n"))
	return nil
}

func run(c *command, ctx context.Context, devConn dev.DevConn) error {
	if c != nil {
		// check required flags
		if err := checkFlags(c.required); err != nil {
			return errors.Trace(err)
		}

		// run the handler
		if err := c.handler(ctx, devConn); err != nil {
			return errors.Annotatef(err, "%s failed", c.name)
		}
		return nil
	}

	// not found
	usage()
	return nil
}

// getCommand returns a pointer to the command which needs to run, or nil if
// there's no such command
func getCommand(str string) *command {
	for _, c := range commands {
		if c.name == str {
			return &c
		}
	}
	return nil
}

func consoleJunkHandler(data []byte) {
	removeNonText(data)
	select {
	case consoleMsgs <- data:
	default:
		// Junk overflow; do nothing
	}
}

func main() {
	seed1 := time.Now().UnixNano()
	seed2, _ := cRand.Int(cRand.Reader, big.NewInt(4000000000))
	mRand.Seed(seed1 ^ seed2.Int64())

	defer glog.Flush()
	go func() {
		time.Sleep(100 * time.Millisecond)
		glog.Flush()
	}()

	consoleMsgs = make(chan []byte, 10)

	initFlags()
	flag.Parse()

	if *chdir != "" {
		if err := os.Chdir(*chdir); err != nil {
			log.Fatal(err)
		}
	}

	osSpecificInit()

	goflag.CommandLine.Parse([]string{}) // Workaround for noise in golang/glog
	pflagenv.Parse(envPrefix)

	glog.Infof("Version: %s", version.Version)
	glog.Infof("Build ID: %s", version.BuildId)
	glog.Infof("Update channel: %s", update.GetUpdateChannel())

	if err := paths.Init(); err != nil {
		log.Fatal(err)
	}

	if err := state.Init(); err != nil {
		log.Fatal(err)
	}

	if err := update.Init(); err != nil {
		log.Fatal(err)
	}

	consoleInit()

	if len(flag.Args()) == 0 || flag.Arg(0) == "ui" {
		isUI = true
		aws.IsUI = true
	}

	ctx := context.Background()
	var devConn dev.DevConn

	cmd := &commands[0]
	if !isUI {
		cmd = getCommand(flag.Arg(0))
	}
	if cmd != nil && cmd.needDevConn == Yes {
		var err error
		devConn, err = devutil.CreateDevConnFromFlags(ctx)
		if err != nil {
			fmt.Println(errors.Trace(err))
			os.Exit(1)
		}
	}

	if cmd == nil {
		fmt.Fprintf(os.Stderr, "Unknown command: %s. Run \"mos help\"\n", flag.Arg(0))
		os.Exit(1)
	}

	err := run(cmd, ctx, devConn)
	if devConn != nil {
		devConn.Disconnect(context.Background())
	}
	if err != nil {
		glog.Infof("Error: %+v", errors.ErrorStack(err))
		fmt.Fprintf(os.Stderr, "Error: %s\n", errors.ErrorStack(err))
		glog.Flush()
		os.Exit(1)
	}
}
