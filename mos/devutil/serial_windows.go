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
package devutil

import (
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func EnumerateSerialPorts() []string {
	emptyList := []string{}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DEVICEMAP\SERIALCOMM\`, registry.QUERY_VALUE)
	if err != nil {
		return emptyList
	}
	defer k.Close()
	list, err := k.ReadValueNames(0)
	if err != nil {
		return emptyList
	}
	vsm := make([]string, len(list))
	for i, v := range list {
		val, _, _ := k.GetStringValue(v)
		vsm[i] = val
	}
	sort.Strings(vsm)
	return vsm
}

func getCOMNumber(port string) int {
	if !strings.HasPrefix(port, "COM") {
		return -1
	}
	cn, err := strconv.Atoi(port[3:])
	if err != nil {
		return -1
	}
	return cn
}

type byCOMNumber []string

func (a byCOMNumber) Len() int      { return len(a) }
func (a byCOMNumber) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byCOMNumber) Less(i, j int) bool {
	cni := getCOMNumber(a[i])
	cnj := getCOMNumber(a[j])
	if cni < 0 || cnj < 0 {
		return a[i] < a[j]
	}
	return cni < cnj
}

func getDefaultPort() string {
	ports := EnumerateSerialPorts()
	var filteredPorts []string
	for _, p := range ports {
		// COM1 and COM2 are commonly mapped to on-board serial ports which are usually not a good guess.
		if p != "COM1" && p != "COM2" {
			filteredPorts = append(filteredPorts, p)
		}
	}
	if len(filteredPorts) == 0 {
		return ""
	}
	return filteredPorts[0]
}
