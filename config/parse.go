package config

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/boz/ephemerald/lifecycle"
	"github.com/buger/jsonparser"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
)

const (
	maxSize = 200
)

func ReadFile(log logrus.FieldLogger, fpath string) ([]*Pool, error) {
	file, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	switch path.Ext(fpath) {
	case ".yml", ".yaml":
		return ReadYAML(log, file)
	case ".json":
		return Read(log, file)
	default:
		return nil, fmt.Errorf("Unknown extension %v", path.Ext(fpath))
	}
}

func Read(log logrus.FieldLogger, r io.Reader) ([]*Pool, error) {
	var configs []*Pool
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return configs, err
	}
	return ParseAll(log, buf)
}

func ReadYAML(log logrus.FieldLogger, r io.Reader) ([]*Pool, error) {
	var configs []*Pool
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return configs, err
	}

	buf, err = yaml.YAMLToJSON(buf)
	if err != nil {
		return configs, err
	}
	return ParseAll(log, buf)
}

func ParseAll(log logrus.FieldLogger, buf []byte) ([]*Pool, error) {
	var configs []*Pool
	err := jsonparser.ObjectEach(buf, func(key []byte, buf []byte, dt jsonparser.ValueType, _ int) error {
		config, err := Parse(log, string(key), buf)
		if err != nil {
			return err
		}
		configs = append(configs, config)
		return nil
	}, "pools")
	return configs, err
}

func Parse(log logrus.FieldLogger, name string, buf []byte) (*Pool, error) {

	log = log.WithField("pool", name).WithField("component", "config.Parse")

	size, err := jsonparser.GetInt(buf, "size")
	if err != nil {
		log.WithError(err).Error("parsing size")
		return nil, err
	}
	if size <= 0 || size >= maxSize {
		err := fmt.Errorf("invalid pool size %v not in (0,%v)", size, maxSize)
		log.WithError(err).Error("parsing size")
		return nil, err
	}

	image, err := jsonparser.GetString(buf, "image")
	if err != nil {
		log.WithError(err).Error("parsing image")
		return nil, err
	}

	port, err := jsonparser.GetInt(buf, "port")
	if err != nil {
		log.WithError(err).Error("parsing port")
		return nil, err
	}

	contBuf, vt, _, err := jsonparser.Get(buf, "container")
	if vt == jsonparser.NotExist && err == jsonparser.KeyPathNotFoundError {
		contBuf = []byte("{}")
	} else if err != nil {
		log.WithError(err).Error("invalid params type")
		return nil, err
	}

	cont := NewContainer()
	err = json.Unmarshal(contBuf, cont)
	if err != nil {
		return nil, err
	}

	actionBuf, vt, _, err := jsonparser.Get(buf, "actions")
	if vt == jsonparser.NotExist && err == jsonparser.KeyPathNotFoundError {
		actionBuf = []byte("{}")
	} else if err != nil {
		log.WithError(err).Error("invalid actions type")
		return nil, err
	}

	lifecycle := lifecycle.NewManager(log)
	if err := lifecycle.ParseConfig(actionBuf); err != nil {
		log.WithError(err).Error("parsing lifecycle")
		return nil, err
	}

	return &Pool{
		Name:      name,
		Size:      int(size),
		Image:     image,
		Port:      int(port),
		Container: cont,
		Actions:   lifecycle,
	}, nil
}
