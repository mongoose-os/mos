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
package pflagenv

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// ParseFlagSet iterates through all non-set flags in the given FlagSet,
// checks if there is an environment variable with the uppercased flag name
// prepended with the given envPrefix, and if so, sets flag value to the
// environment variable value.
//
// It should be called after Parse is called for the given FlagSet.
func ParseFlagSet(fs *pflag.FlagSet, envPrefix string) {

	// Unfortunately, flag package does not provide a way to distinguish between
	// a flag set to default value and a flag which was not set at all. So
	// here is a workaround: first, we visit all flags and save their names,
	// then we visit all set flags and remove those names.

	nonset := make(map[string]*pflag.Flag)

	fs.VisitAll(func(f *pflag.Flag) {
		nonset[f.Name] = f
	})
	fs.Visit(func(f *pflag.Flag) {
		delete(nonset, f.Name)
	})

	// Now, for each nonset flag, check if there is a corresponding environment
	// variable, and use it.

	setFromEnv(nonset, envPrefix)
}

// The same as ParseFlagSet, but operates on a default FlagSet: pflag.CommandLine
func Parse(envPrefix string) {
	ParseFlagSet(pflag.CommandLine, envPrefix)
}

func setFromEnv(nonset map[string]*pflag.Flag, envPrefix string) {
	for name, f := range nonset {
		envVar := os.Getenv(getEnvName(name, envPrefix))
		if envVar != "" {
			f.Value.Set(envVar)
			f.Changed = true
		}
	}
}

func getEnvName(flagName, envPrefix string) string {
	flagName = strings.ToUpper(flagName)
	flagName = strings.Replace(flagName, "-", "_", -1)
	return fmt.Sprint(envPrefix, flagName)
}
