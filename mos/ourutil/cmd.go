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
package ourutil

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/juju/errors"
)

type CmdOutMode int

const (
	CmdOutNever CmdOutMode = iota
	CmdOutAlways
	CmdOutOnError
)

// RunCmd prints the command it's about to execute, and executes it, with
// stdout and stderr set to those of the current process.
func RunCmdWithInput(input io.Reader, outMode CmdOutMode, args ...string) error {
	Reportf("Running %s", strings.Join(args, " "))

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = input
	var so, se io.ReadCloser
	switch outMode {
	case CmdOutNever:
		// Nothing
	case CmdOutAlways:
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	case CmdOutOnError:
		so, _ = cmd.StdoutPipe()
		se, _ = cmd.StderrPipe()
	}

	if err := cmd.Start(); err != nil {
		return errors.Trace(err)
	}

	var soData, seData []byte
	if so != nil && se != nil {
		soData, _ = ioutil.ReadAll(so)
		seData, _ = ioutil.ReadAll(se)
	}

	if err := cmd.Wait(); err != nil {
		if so != nil && se != nil {
			os.Stdout.Write(soData)
			os.Stderr.Write(seData)
		}
		return errors.Trace(err)
	}

	return nil
}

func RunCmd(outMode CmdOutMode, args ...string) error {
	return RunCmdWithInput(nil, outMode, args...)
}

func GetCommandOutput(command string, args ...string) (string, error) {
	Reportf("Running %s %s", command, strings.Join(args, " "))
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Annotatef(err, "failed to run %s %s", command, args)
	}
	return string(output), nil
}
