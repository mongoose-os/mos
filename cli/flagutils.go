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
	goflag "flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/juju/errors"
	flag "github.com/spf13/pflag"

	"github.com/mongoose-os/mos/common/multierror"
	"github.com/mongoose-os/mos/cli/update"
	"github.com/mongoose-os/mos/version"
)

var (
	hiddenFlags = []string{
		"alsologtostderr",
		"log_backtrace_at",
		"log_dir",
		"logbufsecs",
		"logtostderr",
		"stderrthreshold",
		"v",
		"vmodule",
	}
)

func initFlags() {
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	hideFlags()
	flag.Usage = usage
}

func hideFlags() {
	for _, f := range hiddenFlags {
		flag.CommandLine.MarkHidden(f)
	}
}

func unhideFlags() {
	for _, f := range hiddenFlags {
		f := flag.Lookup(f)
		if f != nil {
			f.Hidden = false
		}
	}
}

func checkFlags(fs []string) error {
	var errs error
	for _, req := range fs {
		f := flag.Lookup(req)
		if f == nil || !f.Changed {
			errs = multierror.Append(errs, errors.Errorf("--%s is required\t\t%s", f.Name, f.Usage))
		}
	}
	return errors.Trace(errs)
}

func printFlag(w io.Writer, opt string, name string) {
	f := flag.Lookup(name)
	arg := "<string>"
	if f.Value.Type() == "bool" {
		arg = ""
	}
	fmt.Fprintf(w, "  --%s %s\t%s. %s, default value: %q\n", name, arg, f.Usage, opt, f.DefValue)
}

func usage() {
	w := tabwriter.NewWriter(os.Stderr, 0, 0, 1, ' ', 0)

	if len(os.Args) == 3 && os.Args[1] == "help" {
		for _, c := range commands {
			if c.name == os.Args[2] {
				fmt.Fprintf(w, "%s %s FLAGS\n", os.Args[0], os.Args[2])
				fmt.Fprintf(w, "\nFlags:\n")
				for _, name := range c.required {
					printFlag(w, "Required", name)
				}
				for _, name := range c.optional {
					printFlag(w, "Optional", name)
				}
				w.Flush()
				os.Exit(1)
			}
		}
	}

	fmt.Fprintf(w, "The Mongoose OS command line tool %s.\n", version.Version)

	if !version.LooksLikeDistrBuildId(version.BuildId) {
		fmt.Fprintf(w, "Update channel: %q. Checking updates... ", update.GetUpdateChannel())
		w.Flush()

		serverVersion, err := update.GetServerMosVersion(string(update.GetUpdateChannel()))
		if err != nil {
			color.New(color.FgRed).Fprintf(w, "Failed to check server version: %s\n", err)
		} else {
			if serverVersion.BuildId != version.BuildId {
				color.New(color.FgRed).Fprintf(
					w, "\nOut of date: new version available: %s\nPlease run \"mos update\"\n",
					serverVersion.BuildVersion,
				)
			} else {
				color.New(color.FgGreen).Fprintf(w, "Up to date.\n")
			}
		}
	}

	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  %s <command>\n", os.Args[0])
	fmt.Fprintf(w, "\nCommands:\n")

	for _, c := range commands {
		if c.extended && !*helpFull {
			continue
		}
		fmt.Fprintf(w, "  %s\t\t%s\n", c.name, c.short)
	}

	fmt.Fprintf(w, "\nGlobal Flags:\n")
	if *helpFull {
		fmt.Fprintf(w, flag.CommandLine.FlagUsages())
	} else {
		printFlag(w, "Optional", "verbose")
		printFlag(w, "Optional", "logtostderr")
	}

	w.Flush()
}
