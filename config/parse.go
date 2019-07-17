package config

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/ghodss/yaml"
)

func ReadFile(fpath string, obj interface{}) error {
	file, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer file.Close()

	switch path.Ext(fpath) {
	case ".yml", ".yaml":
		return ReadYAML(file, obj)
	case ".json":
		return Read(file, obj)
	default:
		return fmt.Errorf("Unknown extension %v", path.Ext(fpath))
	}
}

func Read(r io.Reader, obj interface{}) error {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return Parse(buf, obj)
}

func ReadYAML(r io.Reader, obj interface{}) error {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	buf, err = yaml.YAMLToJSON(buf)
	if err != nil {
		return err
	}
	return Parse(buf, obj)
}

func Parse(buf []byte, obj interface{}) error {
	return json.Unmarshal(buf, obj)
}
