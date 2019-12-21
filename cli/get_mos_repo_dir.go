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
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"context"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/interpreter"
	"github.com/mongoose-os/mos/cli/manifest_parser"
)

func getMosRepoDir(ctx context.Context, devConn dev.DevConn) error {
	logWriterStderr = io.MultiWriter(&logBuf, os.Stderr)
	logWriter = io.MultiWriter(&logBuf)
	if *verbose {
		logWriter = logWriterStderr
	}

	cll, err := getCustomLibLocations()
	if err != nil {
		return errors.Trace(err)
	}

	bParams := &buildParams{
		ManifestAdjustments: manifest_parser.ManifestAdjustments{
			Platform: flags.Platform(),
		},
		CustomLibLocations: cll,
	}

	appDir, err := getCodeDirAbs()
	if err != nil {
		return errors.Trace(err)
	}

	interp := interpreter.NewInterpreter(newMosVars())

	manifest, _, err := manifest_parser.ReadManifest(appDir, &manifest_parser.ManifestAdjustments{
		Platform: bParams.Platform,
	}, interp)
	if err != nil {
		return errors.Trace(err)
	}

	mosDirEffective, err := getMosDirEffective(manifest.MongooseOsVersion, time.Hour*99999)
	if err != nil {
		return errors.Trace(err)
	}

	mosDirEffectiveAbs, err := filepath.Abs(mosDirEffective)
	if err != nil {
		return errors.Annotatef(err, "getting absolute path of %q", mosDirEffective)
	}

	fmt.Println(mosDirEffectiveAbs)
	return nil
}
