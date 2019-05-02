package params_test

import (
	"testing"

	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/types"
	"github.com/stretchr/testify/assert"
)

func Test_Var(t *testing.T) {

	cfg := map[string]string{
		"a": "a-value",
		"b": "{{.Host}}",
		"c": `{{.Var "a"}}:{{.Var "b"}}`,
		"d": `{{.Var "d"}}`,
		"e": `{{.Var "f"}}`,
		"f": `{{.Var "e"}}`,
	}

	p := params.Create(types.Instance{
		Host: "foo",
		Port: "8080",
	}, cfg)

	{
		val, err := p.Var("a")
		assert.NoError(t, err)
		assert.Equal(t, "a-value", val)
	}

	{
		val, err := p.Var("b")
		assert.NoError(t, err)
		assert.Equal(t, "foo", val)
	}

	{
		val, err := p.Var("c")
		assert.NoError(t, err)
		assert.Equal(t, "a-value:foo", val)
	}

	{
		_, err := p.Var("d")
		assert.Error(t, err)
	}

	{
		_, err := p.Var("e")
		assert.Error(t, err)
	}

	{
		_, err := p.Var("f")
		assert.Error(t, err)
	}

}
