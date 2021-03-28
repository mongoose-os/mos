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
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"context"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/build"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/interpreter"
	"github.com/mongoose-os/mos/cli/manifest_parser"
	flag "github.com/spf13/pflag"
)

func evalManifestExpr(ctx context.Context, devConn dev.DevConn) error {
	cll, err := getCustomLocations(*flags.Libs)
	if err != nil {
		return errors.Trace(err)
	}
	cml, err := getCustomLocations(*flags.Modules)
	if err != nil {
		return errors.Trace(err)
	}

	args := flag.Args()[1:]

	if len(args) == 0 {
		return errors.Errorf("expression is required")
	}

	expr := args[0]

	bParams := &build.BuildParams{
		ManifestAdjustments: build.ManifestAdjustments{
			Platform: flags.Platform(),
		},
		CustomLibLocations:    cll,
		CustomModuleLocations: cml,
		// Never update libs on that command
		LibsUpdateInterval: 0,
	}

	interp := interpreter.NewInterpreter(newMosVars())

	appDir, err := getCodeDirAbs()
	if err != nil {
		return errors.Trace(err)
	}

	logWriterStderr = os.Stderr

	if *flags.Verbose {
		logWriter = logWriterStderr
	} else {
		logWriter = &bytes.Buffer{}
	}

	compProvider := compProviderReal{
		bParams:   bParams,
		logWriter: logWriter,
	}

	buildVarsCli, err := getBuildVarsFromCLI()
	if err != nil {
		return errors.Trace(err)
	}

	manifest, _, err := manifest_parser.ReadManifestFinal(
		appDir, &build.ManifestAdjustments{
			Platform:  bParams.Platform,
			BuildVars: buildVarsCli,
		}, logWriter, interp,
		&manifest_parser.ReadManifestCallbacks{ComponentProvider: &compProvider},
		false /* requireArch */, *flags.PreferPrebuiltLibs, 0, /* binaryLibsUpdateInterval */
	)
	if err != nil {
		return errors.Trace(err)
	}

	if err := interpreter.SetManifestVars(interp.MVars, manifest); err != nil {
		return errors.Trace(err)
	}

	res, err := interp.EvaluateExpr(expr)
	if err != nil {
		return errors.Trace(err)
	}

	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return errors.Trace(err)
	}

	// TODO(dfrank): probably add a flag whether to expand vars (the default
	// being to expand)
	sdata, err := interpreter.ExpandVars(interp, string(data), false)
	if err != nil {
		return errors.Trace(err)
	}

	fmt.Println(sdata)

	return nil
}
