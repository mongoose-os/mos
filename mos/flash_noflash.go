// +build noflash

package main

import "github.com/cesanta/errors"

func flash() error {
	return errors.NotImplementedf("this build was built without flashing support")
}

func esp32EncryptImage() error {
	return errors.NotImplementedf("this build was built without flashing support")
}
