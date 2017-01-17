// +build windows

package main

import "golang.org/x/sys/windows/registry"

func enumerateSerialPorts() []string {
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
	return vsm
}
