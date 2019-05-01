package ourio

import (
	"bytes"
	"io/ioutil"
	"os"

	"github.com/cesanta/errors"
	yaml "gopkg.in/yaml.v2"
)

// WriteFileIfDiffers writes data to file but avoids overwriting a file with the same contents.
// Returns true if existing file was updated.
func WriteFileIfDifferent(filename string, data []byte, perm os.FileMode) (bool, error) {
	exData, err := ioutil.ReadFile(filename)

	if err == nil && bytes.Compare(exData, data) == 0 {
		return false, nil
	}

	if err2 := ioutil.WriteFile(filename, data, perm); err2 != nil {
		return false, errors.Trace(err2)
	}

	return (err == nil), nil
}

// WriteFileIfDiffers writes s as YAML to file but avoids overwriting a file with the same contents.
// Returns true if the file was updated.
func WriteYAMLFileIfDifferent(filename string, s interface{}, perm os.FileMode) (bool, error) {
	data, err := yaml.Marshal(s)
	if err != nil {
		return false, errors.Trace(err)
	}
	return WriteFileIfDifferent(filename, data, perm)
}
