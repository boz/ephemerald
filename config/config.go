package config

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/ui"
	"github.com/buger/jsonparser"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
)

const (
	maxSize = 200
)

type Config struct {
	Name      string
	Size      int
	Image     string
	Port      int
	Container *Container
	// Params    params.Config
	Lifecycle lifecycle.Manager

	log logrus.FieldLogger

	uie ui.PoolEmitter
}

func ReadFile(log logrus.FieldLogger, uie ui.Emitter, fpath string) ([]*Config, error) {
	file, err := os.Open(fpath)
	if err != nil {
		return []*Config{}, err
	}
	defer file.Close()

	switch path.Ext(fpath) {
	case ".yml", ".yaml":
		return ReadYAML(log, uie, file)
	case ".json":
		return Read(log, uie, file)
	default:
		return nil, fmt.Errorf("Unknown extension %v", path.Ext(fpath))
	}
}

func Read(log logrus.FieldLogger, uie ui.Emitter, r io.Reader) ([]*Config, error) {
	var configs []*Config
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return configs, err
	}
	return ParseAll(log, uie, buf)
}

func ReadYAML(log logrus.FieldLogger, uie ui.Emitter, r io.Reader) ([]*Config, error) {
	var configs []*Config
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return configs, err
	}

	buf, err = yaml.YAMLToJSON(buf)
	if err != nil {
		return configs, err
	}
	return ParseAll(log, uie, buf)
}

func ParseAll(log logrus.FieldLogger, uie ui.Emitter, buf []byte) ([]*Config, error) {
	var configs []*Config
	err := jsonparser.ObjectEach(buf, func(key []byte, buf []byte, dt jsonparser.ValueType, _ int) error {
		config, err := Parse(log, uie, string(key), buf)
		if err != nil {
			return err
		}
		configs = append(configs, config)
		return nil
	}, "pools")
	return configs, err
}

func Parse(log logrus.FieldLogger, uie ui.Emitter, name string, buf []byte) (*Config, error) {

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

	// paramBuf, vt, _, err := jsonparser.Get(buf, "params")
	// if vt == jsonparser.NotExist && err == jsonparser.KeyPathNotFoundError {
	// 	paramBuf = []byte("{}")
	// } else if err != nil {
	// 	log.WithError(err).Error("invalid params type")
	// 	return nil, err
	// }

	// TODO
	// params, err := params.ParseConfig(paramBuf)
	// if err != nil {
	// 	log.WithError(err).Error("parsing params")
	// 	return nil, err
	// }

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

	return &Config{
		Name:      name,
		Size:      int(size),
		Image:     image,
		Port:      int(port),
		Container: cont,
		// Params:    params,
		Lifecycle: lifecycle,
		log:       log,
		uie:       uie.ForPool(name),
	}, nil
}

func (c Config) Log() logrus.FieldLogger {
	return c.log
}

func (c Config) Emitter() ui.PoolEmitter {
	return c.uie
}
