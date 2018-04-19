package debug_core_dump

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"cesanta.com/mos/dev"

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
			image:              "docker.cesanta.com/cc3200-build:1.3.0-r8",
			extraGDBArgs:       []string{},
			extraServeCoreArgs: []string{},
		},
		"cc3220": platformDebugParams{
			image:              "docker.cesanta.com/cc3220-build:2.10.00.04-r2",
			extraGDBArgs:       []string{},
			extraServeCoreArgs: []string{},
		},
		"esp32": platformDebugParams{
			image:              "docker.cesanta.com/esp32-build:3.0-rc1-r8",
			extraGDBArgs:       []string{"-ex", "add-symbol-file /opt/Espressif/rom/rom.elf 0x40000000"},
			extraServeCoreArgs: []string{"--rom=/opt/Espressif/rom/rom.bin", "--rom_addr=0x40000000", "--xtensa_addr_fixup=true"},
		},
		"esp8266": platformDebugParams{
			image:              "docker.cesanta.com/esp8266-build:2.2.1-1.5.0-r2",
			extraGDBArgs:       []string{"-ex", "add-symbol-file /opt/Espressif/rom/rom.elf 0x40000000"},
			extraServeCoreArgs: []string{"--rom=/opt/Espressif/rom/rom.bin", "--rom_addr=0x40000000"},
		},
		"stm32": platformDebugParams{
			image:              "docker.cesanta.com/stm32-build:1.8.0-r5",
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

func getFwELFFile() string {
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
	return ""
}

func GetArchFromCoreDump(data []byte) string {
	if cs := bytes.LastIndex(data, []byte(CoreDumpStart)); cs >= 0 {
		data = data[cs+len(CoreDumpStart):]
	}
	if ce := bytes.Index(data, []byte(CoreDumpEnd)); ce >= 0 {
		data = data[:ce]
	}
	data = bytes.Replace(data, []byte("\r"), nil, -1)
	data = bytes.Replace(data, []byte("\n"), nil, -1)
	cdj := map[string]interface{}{}
	if jerr := json.Unmarshal(data, &cdj); jerr == nil {
		if a, ok := cdj["arch"]; ok {
			return a.(string)
		}
	}
	return ""
}

func getArchFromCoreDumpFile(cdFile string) string {
	data, err := ioutil.ReadFile(cdFile)
	if err != nil {
		return ""
	}
	return GetArchFromCoreDump(data)
}

func DebugCoreDumpF(cdFile, elfFile, platform string, traceOnly bool) error {
	if cdFile == "" {
		return errors.Errorf("cdFile is required")
	}
	if platform == "" {
		platform = getArchFromCoreDumpFile(cdFile)
		if platform == "" {
			return errors.Errorf("--platform is not set and could not be guessed")
		}
	}
	ourutil.Reportf("Using platform: %s", platform)
	dp, ok := debugParams[strings.ToLower(platform)]
	if !ok {
		return errors.Errorf("don't know how to handle %q", platform)
	}
	if elfFile == "" {
		elfFile = getFwELFFile()
		if elfFile == "" {
			return errors.Errorf("--fw-elf-file is not set and could not be guessed")
		}
	}
	ourutil.Reportf("Using ELF file at: %s", elfFile)
	dockerImage := dp.image
	ourutil.Reportf("Using Docker image: %s", dockerImage)
	cmd := []string{"docker", "run", "--rm"}
	if !traceOnly {
		cmd = append(cmd, "-i", "--tty=true")
	}
	cmd = append(cmd,
		"-v", fmt.Sprintf("%s:/fw.elf", elfFile),
		"-v", fmt.Sprintf("%s:/core", cdFile),
	)
	mosSrcPath := getMosSrcPath()
	if mosSrcPath != "" {
		ourutil.Reportf("Using Mongoose OS souces at: %s", mosSrcPath)
		cmd = append(cmd, "-v", fmt.Sprintf("%s:/mongoose-os", mosSrcPath))
	}
	if cwd, err := os.Getwd(); err == nil {
		cmd = append(cmd, "-v", fmt.Sprintf("%s:%s", cwd, filepath.ToSlash(cwd)))
	}
	cmd = append(cmd, dockerImage)
	shellCmd := []string{"/usr/local/bin/serve_core.py"}
	shellCmd = append(shellCmd, dp.extraServeCoreArgs...)
	shellCmd = append(shellCmd, []string{"/fw.elf", "/core", "&"}...)
	shellCmd = append(shellCmd, []string{
		"$MGOS_TARGET_GDB", // Defined in the Docker build image.
		"/fw.elf",
		"-ex", "'target remote 127.0.0.1:1234'",
		"-ex", "'set confirm off'",
		"-ex", "bt",
	}...)
	var input io.Reader
	if traceOnly {
		shellCmd = append(shellCmd, []string{"-ex", "quit"}...)
	} else {
		input = os.Stdin
	}
	cmd = append(cmd, "bash", "-c", strings.Join(shellCmd, " "))
	return ourutil.RunCmdWithInput(input, ourutil.CmdOutAlways, cmd...)
}

func DebugCoreDump(ctx context.Context, _ *dev.DevConn) error {
	args := flag.Args()
	var coreFile, elfFile string
	if len(args) < 2 {
		return errors.Errorf("core dump file name is required")
	}
	coreFile = args[1]
	if len(args) > 2 {
		elfFile = args[2]
	}
	return DebugCoreDumpF(coreFile, elfFile, "", false /* traceOnly */)
}
