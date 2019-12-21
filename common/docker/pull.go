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
package docker

import (
	"context"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/juju/errors"
)

// Pull pulls an image
func Pull(ctx context.Context, image string) error {
	cli, err := docker.NewClientFromEnv()
	if err != nil {
		return errors.Trace(err)
	}

	auths, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		return errors.Trace(err)
	}

	comps := strings.Split(image, "/")
	auth := auths.Configs[comps[0]]

	glog.Infof("Pulling image: image=%q", image)
	err = cli.PullImage(docker.PullImageOptions{
		Repository: image,
	}, auth)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}
