package lifecycle

import (
	"fmt"

	"github.com/buger/jsonparser"
)

type Config struct {
	Live  Generator
	Init  Generator
	Reset Generator
}

func (c *Config) UnmarshalJSON(buf []byte) error {
	{
		ac, err := parseGenerator(buf, "live")
		if err != nil {
			return parseError("live", err)
		}
		c.Live = ac
	}
	{
		ac, err := parseGenerator(buf, "init")
		if err != nil {
			return parseError("init", err)
		}
		c.Init = ac
	}
	{
		ac, err := parseGenerator(buf, "reset")
		if err != nil {
			return parseError("reset", err)
		}
		c.Reset = ac
	}
	return nil
}

func parseGenerator(buf []byte, key string) (Generator, error) {
	vbuf, vt, _, err := jsonparser.Get(buf, key)
	if vt == jsonparser.NotExist && err == jsonparser.KeyPathNotFoundError {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	switch vt {
	case jsonparser.Object:
		return ParseGenerator(vbuf)
	default:
		return nil, fmt.Errorf("lifecycle manager: invalid config at %v: ", key)
	}
}

func ParseGenerator(buf []byte) (Generator, error) {
	t, err := jsonparser.GetString(buf, "type")
	if err != nil {
		return nil, parseError("type", err)
	}

	p, err := lookupPlugin(t)
	if err != nil {
		return nil, err
	}
	return p.ParseConfig(buf)
}
