package moscommon

import (
	"fmt"
	"path/filepath"

	flag "github.com/spf13/pflag"
)

var (
	genDirFlag        = flag.String("gen-dir", "", "Directory to put build output under. Default is build_dir/gen")
	binaryLibsDirFlag = flag.String("binary-libs-dir", "", "Directory to put binary libs under. Default is build_dir/objs")
)

func GetBuildDir(projectDir string) string {
	return filepath.Join(projectDir, "build")
}

func GetManifestFilePath(projectDir string) string {
	return filepath.Join(projectDir, "mos.yml")
}

func GetManifestArchFilePath(projectDir, arch string) string {
	return filepath.Join(projectDir, fmt.Sprintf("mos_%s.yml", arch))
}

func GetGeneratedFilesDir(buildDir string) string {
	if *genDirFlag != "" {
		if gdfa, err := filepath.Abs(*genDirFlag); err == nil {
			return gdfa
		}
	}
	return filepath.Join(buildDir, "gen")
}

func GetObjectDir(buildDir string) string {
	return filepath.Join(buildDir, "objs")
}

func GetFirmwareDir(buildDir string) string {
	return filepath.Join(buildDir, "fw")
}

func GetFilesystemStagingDir(buildDir string) string {
	return filepath.Join(buildDir, "fs")
}

func GetPlatformMakefilePath(mosDir, platform string) string {
	return filepath.Join(mosDir, "fw", "platforms", platform, "Makefile.build")
}

func GetBuildCtxFilePath(buildDir string) string {
	return filepath.Join(GetGeneratedFilesDir(buildDir), "build_ctx.txt")
}

func GetBuildStatFilePath(buildDir string) string {
	return filepath.Join(GetGeneratedFilesDir(buildDir), "build_stat.json")
}

func GetMakeVarsFilePath(buildDir string) string {
	return filepath.Join(GetGeneratedFilesDir(buildDir), "vars.mk")
}

func GetFirmwareElfFilePath(buildDir string) string {
	return filepath.Join(GetObjectDir(buildDir), "fw.elf")
}

func GetOrigLibArchiveFilePath(buildDir, platform string) string {
	if platform == "esp32" {
		return filepath.Join(GetObjectDir(buildDir), "moslib", "libmoslib.a")
	} else {
		return filepath.Join(GetObjectDir(buildDir), "lib.a")
	}
}

func GetLibArchiveFilePath(buildDir string) string {
	return filepath.Join(buildDir, "lib.a")
}

func GetFirmwareZipFilePath(buildDir string) string {
	return filepath.Join(buildDir, "fw.zip")
}

func GetBuildLogFilePath(buildDir string) string {
	return filepath.Join(buildDir, "build.log")
}

func GetBuildLogLocalFilePath(buildDir string) string {
	return filepath.Join(buildDir, "build.local.log")
}

func GetMosFinalFilePath(buildDir string) string {
	return filepath.Join(GetGeneratedFilesDir(buildDir), "mos_final.yml")
}

func GetDepsInitCFilePath(buildDir string) string {
	return filepath.Join(GetGeneratedFilesDir(buildDir), "mgos_deps_init.c")
}

func GetConfSchemaFilePath(buildDir string) string {
	return filepath.Join(GetGeneratedFilesDir(buildDir), "mos_conf_schema.yml")
}

func GetBinaryLibFilePath(buildDir, name, variant, version string) string {
	bld := *binaryLibsDirFlag
	if bld == "" {
		bld = GetObjectDir(buildDir)
	}
	return filepath.Join(bld, fmt.Sprintf("lib%s-%s-%s.a", name, variant, version))
}

func GetSdkVersionFile(mosDir, platform string) string {
	return filepath.Join(mosDir, "fw", "platforms", platform, "sdk.version")
}
