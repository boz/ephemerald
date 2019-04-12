package instance

import (
	"errors"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/types"
)

type Config struct {
	ID        types.ID
	Container config.Container
}

func NewConfig(pcfg config.Pool) (Config, error) {
	return Config{}, errors.New("not implemented")
}
