package devutil

import (
	"path/filepath"
	"sort"
	"strings"
)

func EnumerateSerialPorts() []string {
	list, _ := filepath.Glob("/dev/cu.*")
	var filteredList []string
	for _, s := range list {
		if !strings.Contains(s, "Bluetooth-") &&
			!strings.Contains(s, "-SPPDev") &&
			!strings.Contains(s, "-WirelessiAP") {
			filteredList = append(filteredList, s)
		}
	}
	sort.Strings(filteredList)
	return filteredList
}
