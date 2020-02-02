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
package moscommon

import (
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"
)

var (
	buildDirFlag      = flag.String("build-dir", "", "Directory to put build output under. Default is project_dir/build")
	genDirFlag        = flag.String("gen-dir", "", "Directory to put generated output under. Default is build_dir/gen")
	binaryLibsDirFlag = flag.String("binary-libs-dir", "", "Directory to put binary libs under. Default is build_dir/objs")
)

func GetBuildDir(projectDir string) string {
	if *buildDirFlag != "" {
		if bdfa, err := filepath.Abs(*buildDirFlag); err == nil {
			return bdfa
		}
	}
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
	// New repo layout introduced on 2019/04/29, current release is 2.13.1.
	oldPath := filepath.Join(mosDir, "fw", "platforms", platform, "Makefile.build")
	newPath := filepath.Join(mosDir, "platforms", platform, "Makefile.build")
	if _, err := os.Stat(newPath); err == nil {
		return newPath
	}
	return oldPath
}

func GetSdkVersionFile(mosDir, platform string) string {
	// New repo layout introduced on 2019/04/29, current release is 2.13.1.
	oldPath := filepath.Join(mosDir, "fw", "platforms", platform, "sdk.version")
	newPath := filepath.Join(mosDir, "platforms", platform, "sdk.version")
	if _, err := os.Stat(newPath); err == nil {
		return newPath
	}
	return oldPath
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
