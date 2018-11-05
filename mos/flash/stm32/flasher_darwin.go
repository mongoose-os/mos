package stm32

import (
	"github.com/cesanta/errors"
)

func GetSTLinkMountForPort(port string) (string, error) {
	// TODO(rojer)
	return "", errors.NotImplementedf("GetSTLinkMountForPort")
}

func GetSTLinkMounts() ([]string, error) {
	return getSTLinkMountsInDir("/Volumes")
}
