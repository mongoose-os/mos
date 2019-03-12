package debug_core_dump

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"cesanta.com/mos/dev"
	"cesanta.com/mos/flags"

	"cesanta.com/common/go/ourutil"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

const (
	CoreDumpStart = "--- BEGIN CORE DUMP ---"
	CoreDumpEnd   = "---- END CORE DUMP ----"
)

type platformDebugParams struct {
	image              string
	extraGDBArgs       []string
	extraServeCoreArgs []string
}

// In the future we will set all the necessary parameters in the build image itself.
// For now, we have this lookup table.
var (
	debugParams = map[string]platformDebugParams{
		"cc3200": platformDebugParams{
			image:              "docker.io/mgos/cc3200-build:1.3.0-r11",
			extraGDBArgs:       []string{},
			extraServeCoreArgs: []string{},
		},
		"cc3220": platformDebugParams{
			image:              "docker.io/mgos/cc3220-build:2.10.00.04-r4",
			extraGDBArgs:       []string{},
			extraServeCoreArgs: []string{},
		},
		"esp32": platformDebugParams{
			image:              "docker.io/mgos/esp32-build:3.2-r3",
			extraGDBArgs:       []string{"-ex", "add-symbol-file /opt/Espressif/rom/rom.elf 0x40000000"},
			extraServeCoreArgs: []string{"--rom=/opt/Espressif/rom/rom.bin", "--rom_addr=0x40000000", "--xtensa_addr_fixup=true"},
		},
		"esp8266": platformDebugParams{
			image:              "docker.io/mgos/esp8266-build:2.2.1-1.5.0-r4",
			extraGDBArgs:       []string{"-ex", "add-symbol-file /opt/Espressif/rom/rom.elf 0x40000000"},
			extraServeCoreArgs: []string{"--rom=/opt/Espressif/rom/rom.bin", "--rom_addr=0x40000000"},
		},
		"stm32": platformDebugParams{
			image:              "docker.io/mgos/stm32-build:r14",
			extraGDBArgs:       []string{},
			extraServeCoreArgs: []string{},
		},
		"rs14100": platformDebugParams{
			image:              "docker.io/mgos/rs14100-build:1.0.4-r1",
			extraGDBArgs:       []string{},
			extraServeCoreArgs: []string{},
		},
	}

	mosSrcPath = ""
	fwELFFile  = ""
)

func init() {
	flag.StringVar(&mosSrcPath, "mos-src-path", "", "Path to mos fw sources")
	flag.StringVar(&fwELFFile, "fw-elf-file", "", "Path to teh firmware ELF file")
}

func getMosSrcPath() string {
	if mosSrcPath != "" {
		return mosSrcPath
	}
	// Try a few guesses
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// Are we in the app dir? Check if mos is among the deps.
	try := filepath.Join(cwd, "deps", "mongoose-os")
	glog.V(2).Infof("Trying %q", try)
	if _, err := os.Stat(try); err == nil {
		return try
	}
	// Try going up - maybe we are in a repo that includes mos (like our dev repo).
	for dir := cwd; ; {
		file := ""
		dir, file = filepath.Split(dir)
		if file == "" {
			break
		}
		dir = filepath.Clean(dir)
		try = filepath.Join(dir, "fw")
		glog.V(2).Infof("Trying %q %q", try, dir)
		if _, err := os.Stat(try); err == nil {
			return dir
		}
	}
	return ""
}

func getFwELFFile(app, platform, version, buildID string) string {
	if fwELFFile != "" {
		return fwELFFile
	}
	// Try a few guesses
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// Are we in the app dir? Use file from build dir.
	try := filepath.Join(cwd, "build", "objs", "fw.elf")
	glog.V(2).Infof("Trying %q", try)
	if _, err := os.Stat(try); err == nil {
		return try
	}
	try = filepath.Join(cwd, "build", "objs", fmt.Sprintf("%s.elf", app))
	glog.V(2).Infof("Trying %q", try)
	if _, err := os.Stat(try); err == nil {
		return try
	}
	return ""
}

type CoreDumpInfo struct {
	App        string `json:"app"`
	Platform   string `json:"arch"`
	Version    string `json:"version"`
	BuildID    string `json:"build_id"`
	BuildImage string `json:"build_image"`
}

func GetInfoFromCoreDump(data []byte) (CoreDumpInfo, error) {
	if cs := bytes.LastIndex(data, []byte(CoreDumpStart)); cs >= 0 {
		data = data[cs+len(CoreDumpStart):]
	}
	if ce := bytes.Index(data, []byte(CoreDumpEnd)); ce >= 0 {
		data = data[:ce]
	}
	data = bytes.Replace(data, []byte("\r"), nil, -1)
	data = bytes.Replace(data, []byte("\n"), nil, -1)
	var info CoreDumpInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return info, errors.Annotatef(err, "core dump is not valid JSON object")
	}
	return info, nil
}

func GetInfoFromCoreDumpFile(cdFile string) (CoreDumpInfo, error) {
	data, err := ioutil.ReadFile(cdFile)
	if err != nil {
		return CoreDumpInfo{}, errors.Annotatef(err, "error reading file")
	}
	return GetInfoFromCoreDump(data)
}

func DebugCoreDumpF(cdFile, elfFile string, traceOnly bool) error {
	var ok bool
	var err error
	var info CoreDumpInfo
	if cdFile != "" {
		cdFile2 := cdFile
		cdFile, err = filepath.Abs(cdFile)
		if err != nil {
			return errors.Annotatef(err, "invalid file name %s", cdFile2)
		}
		info, err = GetInfoFromCoreDumpFile(cdFile)
		if err != nil {
			return errors.Annotatef(err, "unable to parse %s", cdFile)
		}
		if info.App != "" {
			ourutil.Reportf("Core dump by %s/%s %s %s", info.App, info.Platform, info.Version, info.BuildID)
		}
	} else {
		info.Platform = flags.Platform()
		if info.Platform == "" {
			return errors.Errorf("--platform is required when running with no dump")
		}
	}
	if elfFile == "" {
		elfFile = getFwELFFile(info.App, info.Platform, info.Version, info.BuildID)
		if elfFile == "" {
			return errors.Errorf("--fw-elf-file is not set and could not be guessed")
		}
	}
	dp, ok := debugParams[strings.ToLower(info.Platform)]
	if !ok {
		return errors.Errorf("don't know how to handle %q", info.Platform)
	}
	elfFile2 := elfFile
	elfFile, err = filepath.Abs(elfFile)
	if err != nil {
		return errors.Annotatef(err, "invalid file name %s", elfFile2)
	}
	if _, err := os.Stat(elfFile); err != nil {
		return errors.Annotatef(err, "invalid file %s", elfFile)
	}
	ourutil.Reportf("Using ELF file at: %s", elfFile)
	dockerImage := info.BuildImage
	if dockerImage == "" {
		dockerImage = dp.image
	}
	ourutil.Reportf("Using Docker image: %s", dockerImage)
	cmd := []string{"docker", "run", "--rm"}
	if !traceOnly {
		cmd = append(cmd, "-i", "--tty=true")
	}
	cmd = append(cmd, "-v", fmt.Sprintf("%s:/fw.elf", ourutil.GetPathForDocker(elfFile)))
	if cdFile != "" {
		cmd = append(cmd, "-v", fmt.Sprintf("%s:/core", ourutil.GetPathForDocker(cdFile)))
	}
	mosSrcPath := getMosSrcPath()
	if mosSrcPath != "" {
		ourutil.Reportf("Using Mongoose OS souces at: %s", mosSrcPath)
		cmd = append(cmd, "-v", fmt.Sprintf("%s:/mongoose-os", ourutil.GetPathForDocker(mosSrcPath)))
	}
	if cwd, err := os.Getwd(); err == nil {
		cmd = append(cmd, "-v", fmt.Sprintf("%s:%s", cwd, ourutil.GetPathForDocker(cwd)))
	}
	cmd = append(cmd, dockerImage)
	input := os.Stdin
	var shellCmd []string
	if cdFile != "" {
		shellCmd = append(shellCmd, "/usr/local/bin/serve_core.py")
		shellCmd = append(shellCmd, dp.extraServeCoreArgs...)
		shellCmd = append(shellCmd, "/fw.elf", "/core", "&")
		shellCmd = append(shellCmd,
			"$MGOS_TARGET_GDB", // Defined in the Docker build image.
			"/fw.elf",
			"-ex", "'target remote 127.0.0.1:1234'",
			"-ex", "'set confirm off'",
			"-ex", "bt",
		)
		if traceOnly {
			shellCmd = append(shellCmd, []string{"-ex", "quit"}...)
			input = nil
		}
	} else {
		shellCmd = append(shellCmd,
			"$MGOS_TARGET_GDB", // Defined in the Docker build image.
			"/fw.elf",
		)
	}
	cmd = append(cmd, "bash", "-c", strings.Join(shellCmd, " "))
	return ourutil.RunCmdWithInput(input, ourutil.CmdOutAlways, cmd...)
}

func DebugCoreDump(ctx context.Context, _ dev.DevConn) error {
	args := flag.Args()
	var coreFile, elfFile string
	if len(args) < 2 {
		return errors.Errorf("core dump file name is required")
	}
	coreFile = args[1]
	if len(args) > 2 {
		elfFile = args[2]
	}
	return DebugCoreDumpF(coreFile, elfFile, false /* traceOnly */)
}
