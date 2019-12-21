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
// +build noflash

package main

import (
	"context"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/dev"
)

func esp32EFuseGet(ctx context.Context, devConn dev.DevConn) error {
	return errors.NotImplementedf("esp32-efuse-get: this build was built without flashing support")
}

func esp32EFuseSet(ctx context.Context, devConn dev.DevConn) error {
	return errors.NotImplementedf("esp32-efuse-set: this build was built without flashing support")
}

func esp32EncryptImage(ctx context.Context, devConn dev.DevConn) error {
	return errors.NotImplementedf("esp32-encrypt-image: this build was built without flashing support")
}

func esp32GenKey(ctx context.Context, devConn dev.DevConn) error {
	return errors.NotImplementedf("esp32-gen-key: this build was built without flashing support")
}

func flash(ctx context.Context, devConn dev.DevConn) error {
	return errors.NotImplementedf("flash: this build was built without flashing support")
}

func flashRead(ctx context.Context, devConn dev.DevConn) error {
	return errors.NotImplementedf("flash-read: this build was built without flashing support")
}
