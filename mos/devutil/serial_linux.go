package devutil

import (
	"path/filepath"
	"sort"
)

func EnumerateSerialPorts() []string {
	// Note: Prefer ttyUSB* to ttyACM*.
	list1, _ := filepath.Glob("/dev/ttyUSB*")
	sort.Strings(list1)
	list2, _ := filepath.Glob("/dev/ttyACM*")
	sort.Strings(list2)
	return append(list1, list2...)
}
