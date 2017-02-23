package cpool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvisionBuilder(t *testing.T) {
	var p interface{}
	var ok bool

	{
		p = BuildProvisioner().Create()

		_, ok = isInitializeProvisioner(p)
		assert.False(t, ok)

		_, ok = isResetProvisioner(p)
		assert.False(t, ok)
	}

	{
		p = BuildProvisioner().
			WithInitialize(func(_ context.Context, _ StatusItem) error {
				return nil
			}).Create()

		_, ok = isInitializeProvisioner(p)
		assert.True(t, ok)

		_, ok = isResetProvisioner(p)
		assert.False(t, ok)
	}

	{
		p = BuildProvisioner().
			WithReset(func(_ context.Context, _ StatusItem) error {
				return nil
			}).Create()

		_, ok = isInitializeProvisioner(p)
		assert.False(t, ok)

		_, ok = isResetProvisioner(p)
		assert.True(t, ok)
	}

}
