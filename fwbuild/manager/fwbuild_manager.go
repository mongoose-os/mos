/*
 * Copyright (c) 2014-2018 Cesanta Software Limited
 * All rights reserved
 *
 * Licensed under the Apache License, Version 2.0 (the ""License"");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an ""AS IS"" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//go:generate go-bindata-assetfs -pkg main -nocompress -modtime 1 -mode 420 web_root/...

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/golang/glog"
	"github.com/juju/errors"
	goji "goji.io"
	"goji.io/pat"

	"github.com/mongoose-os/mos/common/docker"
	fwbuildcommon "github.com/mongoose-os/mos/fwbuild/common"
	"github.com/mongoose-os/mos/fwbuild/common/reqpar"
	"github.com/mongoose-os/mos/fwbuild/manager/middleware"
	"github.com/mongoose-os/mos/version"
)

var (
	instanceDockerImage = flag.String("instance-docker-image", "docker.io/mgos/fwbuild-instance", "Fwbuild instance docker image, without a tag")
	mosImage            = flag.String("mos-image", "docker.io/mgos/mos", "Mos tool docker image, without a tag")
	volumesDir          = flag.String("volumes-dir", "/var/tmp/fwbuild-volumes", "")

	port              = flag.String("port", "80", "HTTP port to listen at.")
	portTLS           = flag.String("port-tls", "443", "HTTPS port to listen at.")
	certFile          = flag.String("cert-file", "", "TLS certificate file")
	keyFile           = flag.String("key-file", "", "TLS key file")
	payloadLimit      = flag.Int64("payload-size-limit", 5*1024*1024, "Max upload size")
	imagePullInterval = flag.Duration("image-pull-interval", 1*time.Hour, "Pull images at this interval")

	errBuildFailure = errors.New("build failure")

	imagePullTimestamp     = map[string]time.Time{}
	imagePullTimestampLock = sync.Mutex{}
)

func main() {
	flag.Parse()

	glog.Infof("fwbuild-manager %s (%s)", version.Version, version.BuildId)

	if err := os.MkdirAll(*volumesDir, 0775); err != nil {
		glog.Fatal(err)
	}

	handler, err := CreateHandler()
	if err != nil {
		glog.Fatal(err)
	}

	var tlsConfig *tls.Config
	if *certFile != "" || *keyFile != "" {
		// Check for partial configuration.
		if *certFile == "" || *keyFile == "" {
			glog.Exitf("Failed to load certificate and key: both were not provided")
			*certFile = ""
			*keyFile = ""
		}
		tlsConfig = &tls.Config{
			MinVersion:               tls.VersionTLS10,
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
			NextProtos:   []string{"http/1.1"},
			Certificates: make([]tls.Certificate, 1),
		}
		glog.Infof("Cert file: %s", *certFile)
		glog.Infof("Key file : %s", *keyFile)
		tlsConfig.Certificates[0], err = tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			glog.Exitf("Failed to load certificate and key: %s", err)
		}

		hs := &http.Server{
			Addr:      fmt.Sprintf(":%s", *portTLS),
			Handler:   handler,
			TLSConfig: tlsConfig,
		}

		go func() {
			glog.Infof("Listening at the HTTPS port %s ...", *portTLS)
			glog.Fatal(hs.ListenAndServeTLS(*certFile, *keyFile))
		}()
	} else {
		glog.Warning("Running without TLS")
	}

	hs := &http.Server{
		Addr:    fmt.Sprintf(":%s", *port),
		Handler: handler,
	}

	glog.Infof("Listening at the HTTP port %s ...", *port)
	glog.Fatal(hs.ListenAndServe())
}

func CreateHandler() (http.Handler, error) {
	rRoot := goji.NewMux()
	rRoot.Use(middleware.MakeLogger())

	rAPI := goji.SubMux()
	rRoot.Handle(pat.New("/api/*"), rAPI)
	rAPI.HandleFunc(pat.New("/fwbuild/:version/:action"), handleFwbuildAction)

	assetInfo := func(path string) (os.FileInfo, error) {
		return os.Stat(path)
	}

	rRoot.Handle(pat.New("/"), http.FileServer(
		&assetfs.AssetFS{
			Asset:     Asset,
			AssetDir:  AssetDir,
			AssetInfo: assetInfo,
			Prefix:    "web_root",
		},
	))

	return rRoot, nil
}

func getImageName(version string) string {
	return fmt.Sprintf("%s:%s", *instanceDockerImage, version)
}

func doPull(ctx context.Context, version string) error {
	for _, image := range []string{getImageName(version), fmt.Sprintf("docker.io/mgos/mos:%s", version)} {
		glog.Infof("Pulling %s...", image)
		if err := docker.Pull(ctx, image); err != nil {
			return errors.Annotatef(err, "error pulling %s", image)
		}
	}
	return nil
}

// runBuild runs fwbuild-instance container with the params reqPar. Returns
// zip data with the build output files; in case of build failure returned
// error is errBuildFailure; this can be used to distinguish build failures
// from other kinds of errors.
func runBuild(ctx context.Context, version string, reqPar *reqpar.RequestParams) ([]byte, error) {
	cmdArgs := []string{
		"--alsologtostderr",
		"--v", flag.Lookup("v").Value.String(),
		"--volumes-dir", path.Join(*volumesDir, version),
		"--mos-image", fmt.Sprintf("%s:%s", *mosImage, version),
	}

	// Create request params json file {{{
	reqParFile, err := ioutil.TempFile(*volumesDir, "req_par_")
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer func() {
		os.RemoveAll(reqParFile.Name())
		reqParFile.Close()
	}()

	parData, err := json.MarshalIndent(reqPar, "", "  ")
	if err != nil {
		return nil, errors.Trace(err)
	}

	if _, err := reqParFile.Write(parData); err != nil {
		return nil, errors.Trace(err)
	}
	// }}}

	// Create output zip file {{{
	outputFile, err := ioutil.TempFile(*volumesDir, "fwbuild_output_zip_")
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer func() {
		os.RemoveAll(outputFile.Name())
		outputFile.Close()
	}()
	// }}}

	imageName := getImageName(version)

	// Pull the image first, if necessary.
	if *imagePullInterval != 0 {
		imagePullTimestampLock.Lock()
		lastPullTimestamp := imagePullTimestamp[imageName]
		if time.Now().After(lastPullTimestamp.Add(*imagePullInterval)) {
			imagePullTimestamp[imageName] = time.Now()
			imagePullTimestampLock.Unlock()
			// If this is the first time, make it blocking.
			// In either case, it's best effort so build does not fail.
			if lastPullTimestamp.IsZero() {
				doPull(ctx, version)
			} else {
				go doPull(ctx, version)
			}
		} else {
			imagePullTimestampLock.Unlock()
		}
	}

	cmdArgs = append(cmdArgs, "--req-params", reqParFile.Name())
	cmdArgs = append(cmdArgs, "--output-zip", outputFile.Name())

	cmdArgs = append(cmdArgs, "build")

	runOpts := []docker.RunOption{
		// Mgos container should be able to spawn other containers
		// (read about the "sibling containers" "approach:
		// https://jpetazzo.github.io/2015/09/03/do-not-use-docker-in-docker-for-ci/)
		docker.Bind("/var/run/docker.sock", "/var/run/docker.sock", "rw"),
	}
	// This is no longer necessary post-2.6 but is preserved for backward compatibility.
	if dockerBin, err := exec.LookPath("docker"); err == nil {
		runOpts = append(runOpts, docker.Bind(dockerBin, "/usr/bin/docker", "ro"))
	}
	runOpts = append(runOpts,
		docker.Bind(*volumesDir, *volumesDir, "rw"),
		docker.Cmd(cmdArgs),
	)

	buildErr := docker.Run(ctx, imageName, os.Stdout, runOpts...)

	// Read zip data from output file
	data, err := ioutil.ReadAll(outputFile)

	// Return data and a proper error (if any)
	if buildErr != nil {
		glog.Errorf("Build error: %+v", errors.ErrorStack(buildErr))
		exitError, ok := errors.Cause(buildErr).(*docker.ExitError)
		if ok && exitError.Code() == fwbuildcommon.FwbuildExitCodeBuildFailed {
			return data, errBuildFailure
		}

		return data, errors.Trace(buildErr)
	}

	return data, nil
}

func handleFwbuildAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	version := pat.Param(r, "version")
	action := pat.Param(r, "action")

	switch action {
	case "build":
		// Get request params to be saved to a json file
		reqPar, err := reqpar.New(r, *volumesDir, *payloadLimit)
		if err != nil {
			glog.Infof("Request error: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error() + "\n"))
			return
		}

		defer func() {
			reqPar.RemoveFiles()
		}()

		// Perform the build
		data, err := runBuild(ctx, version, reqPar)
		if err != nil {
			if errors.Cause(err) == errBuildFailure {
				w.WriteHeader(http.StatusTeapot)
			} else {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}
		}

		w.Write(data)

	case "pull":
		if err := doPull(ctx, version); err != nil {
			glog.Infof("Request error: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error() + "\n"))
			return
		}
		w.Write([]byte("Ok\n"))

	default:
		err := errors.Errorf("wrong action: %q", action)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
}
