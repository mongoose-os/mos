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
package flags

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"time"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/ourutil"
	flag "github.com/spf13/pflag"
)

var (
	// --arch was deprecated at 2017/08/15 and should eventually be removed.
	archOld = flag.String("arch", "", "Deprecated, please use --platform instead")
	Port    = flag.String("port", "auto", "Serial port where the device is connected. "+
		"If set to 'auto', ports on the system will be enumerated and the first will be used.")
	BaudRate       = flag.Int("baud-rate", 115200, "Serial port speed")
	Board          = flag.String("board", "", "Board name.")
	BuildInfo      = flag.String("build-info", "", "")
	Checksums      = flag.StringSlice("checksums", []string{"sha1"}, "")
	Description    = flag.String("description", "", "")
	Input          = flag.StringP("input", "i", "", "")
	Manifest       = flag.String("manifest", "", "")
	Name           = flag.String("name", "", "")
	Output         = flag.StringP("output", "o", "", "")
	platform       = flag.String("platform", "", "Hardware platform")
	SrcDir         = flag.String("src-dir", "", "")
	Compress       = flag.Bool("compress", false, "")
	GHToken        = flag.String("gh-token", "", "GitHub token to use with API and asset reuqests. Can be token itself or @/path/to/file_with_token.")
	ChunkSize      = flag.Int("chunk-size", 512, "Chunk size for operations")
	FsOpAttempts   = flag.Int("fs-op-attempts", 3, "Chunk size for operations")
	PID            = flag.String("pid", "mos", "")
	UID            = flag.String("uid", "", "")
	CertFile       = flag.String("cert-file", "", "Certificate file name")
	KeyFile        = flag.String("key-file", "", "Key file name")
	CAFile         = flag.String("ca-cert-file", "", "CA certificate file name")
	CAKeyFile      = flag.String("ca-key-file", "", "CA key file name (for cert signing)")
	RPCUARTNoDelay = flag.Bool("rpc-uart-no-delay", false, "Do not introduce delay into UART over RPC")
	Timeout        = flag.Duration("timeout", 20*time.Second, "Timeout for the device connection and call operation")
	Reconnect      = flag.Bool("reconnect", false, "Enable reconnection")
	HWFC           = flag.Bool("hw-flow-control", false, "Enable hardware flow control (CTS/RTS)")

	LicenseServer    = flag.String("license-server", "https://license.mongoose-os.com", "License server address")
	LicenseServerKey = flag.String("license-server-key", "", "License server key")

	InvertedControlLines = flag.Bool("inverted-control-lines", false, "DTR and RTS control lines use inverted polarity")
	SetControlLines      = flag.Bool("set-control-lines", true, "Set RTS and DTR explicitly when in console/RPC mode")

	AzureConnectionString = flag.String("azure-connection-string", "", "Azure connection string")

	GCPProject        = flag.String("gcp-project", "", "Google IoT project ID")
	GCPRegion         = flag.String("gcp-region", "", "Google IoT region")
	GCPRegistry       = flag.String("gcp-registry", "", "Google IoT device registry")
	GCPCertFile       = flag.String("gcp-cert-file", "", "Certificate/public key file")
	GCPKeyFile        = flag.String("gcp-key-file", "", "Private key file")
	GCPRPCCreateTopic = flag.Bool("gcp-rpc-create-topic", false, "Create RPC topic plumbing if needed")

	Level    = flag.Int("level", -1, "Config level; default - runtime")
	NoReboot = flag.Bool("no-reboot", false, "Save config but don't reboot the device.")
	NoSave   = flag.Bool("no-save", false, "Don't save config and don't reboot the device")
	TryOnce  = flag.Bool("try-once", false, "When saving the config, do it in such a way that it's only applied on the next boot")

	Format       = flag.String("format", "", "Config format, hex or json")
	WriteKey     = flag.String("write-key", "", "Write key file")
	CSRTemplate  = flag.String("csr-template", "", "CSR template to use")
	CertTemplate = flag.String("cert-template", "", "cert template to use")
	CertDays     = flag.Int("cert-days", 0, "new cert validity, days")
	Subject      = flag.String("subject", "", "Subject for CSR or certificate")

	GDBServerCmd = flag.String("gdb-server-cmd", "/usr/local/bin/serve_core.py", "")

	KeepTempFiles = flag.Bool("keep-temp-files", false, "keep temp files after the build is done (by default they are in ~/.mos/tmp)")
	KeepFS        = flag.Bool("keep-fs", false, "When flashing, skip the filesystem parts")

	Attr      = flag.StringArray("attr", nil, "manifest attribute, can be used multiple times")
	ExtraAttr = flag.StringArray("extra-attr", nil, "manifest extra attribute info to be added to ZIP")

	// Local build flags.
	BuildDockerExtra = flag.StringArray(
		"build-docker-extra", []string{},
		"extra docker flags, added before image name. Can be used multiple times: "+
			"e.g. --build-docker-extra -v --build-docker-extra /foo:/bar.",
	)
	BuildDockerNoMounts = flag.Bool(
		"build-docker-no-mounts", false,
		"if set, then mos will not add bind mounts to the docker invocation. "+
			"For build to work, volumes will need to be provided externally via --build-docker-extra, "+
			"e.g. --build-docker-extra=--volumes-from=outer",
	)
	BuildImage       = flag.String("build-image", "", "Override the Docker image used for build.")
	BuildParalellism = flag.Int("build-parallelism", 0, "build parallelism. default is to use number of CPUs.")
)

func Platform() string {
	if *platform != "" {
		return *platform
	}
	if *archOld != "" {
		ourutil.Reportf("Warning: --arch is deprecated, use --platform")
	}
	return *archOld
}

func TLSConfigFromFlags() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		// TODO(rojer): Ship default CA bundle with mos.
		InsecureSkipVerify: *CAFile == "",
	}

	// Load client cert / key if specified
	if *CertFile != "" && *KeyFile == "" {
		return nil, errors.Errorf("Please specify --key-file")
	}
	if *CertFile != "" {
		cert, err := tls.LoadX509KeyPair(*CertFile, *KeyFile)
		if err != nil {
			return nil, errors.Trace(err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA cert if specified
	if *CAFile != "" {
		caCert, err := ioutil.ReadFile(*CAFile)
		if err != nil {
			return nil, errors.Trace(err)
		}
		tlsConfig.RootCAs = x509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(caCert)
	}

	return tlsConfig, nil
}
