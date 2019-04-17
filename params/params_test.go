package params_test

import (
	"testing"

	"github.com/boz/ephemerald/params"
	"github.com/stretchr/testify/assert"
)

func Test_Get(t *testing.T) {

	cfg := map[string]string{
		"a": "a-value",
		"b": "{{.Host}}",
		"c": `{{.Get "a"}}:{{.Get "b"}}`,
		"d": `{{.Get "d"}}`,
		"e": `{{.Get "f"}}`,
		"f": `{{.Get "e"}}`,
	}

	p := params.Create(params.State{
		Host: "foo",
	}, cfg)

	{
		val, err := p.Get("a")
		assert.NoError(t, err)
		assert.Equal(t, "a-value", val)
	}

	{
		val, err := p.Get("b")
		assert.NoError(t, err)
		assert.Equal(t, "foo", val)
	}

	{
		val, err := p.Get("c")
		assert.NoError(t, err)
		assert.Equal(t, "a-value:foo", val)
	}

	{
		_, err := p.Get("d")
		assert.Error(t, err)
	}

	{
		_, err := p.Get("e")
		assert.Error(t, err)
	}

	{
		_, err := p.Get("f")
		assert.Error(t, err)
	}

}
