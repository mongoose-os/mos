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
package esp8266

// When doing stub development, enable the line below and run:
//  go generate github.com/mongoose-os/mos/cli/flash/esp && go build -v && ./mos flash ...
//
// DISABLED go:generate ./genstubs.sh
//go:generate go-bindata -pkg esp8266 -nocompress -modtime 1 -mode 420 data/
