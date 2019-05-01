// +build !windows

package devutil

func getDefaultPort() string {
	ports := EnumerateSerialPorts()
	if len(ports) == 0 {
		return ""
	}
	return ports[0]
}
