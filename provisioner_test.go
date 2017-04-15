package ephemerald

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

		_, ok = isLiveCheckProvisioner(p)
		assert.False(t, ok)

		_, ok = isInitializeProvisioner(p)
		assert.False(t, ok)

		_, ok = isResetProvisioner(p)
		assert.False(t, ok)
	}

	{ // livecheck
		p = BuildProvisioner().
			WithLiveCheck(func(_ context.Context, _ StatusItem) error {
				return nil
			}).Create()

		_, ok = isLiveCheckProvisioner(p)
		assert.True(t, ok)

		_, ok = isInitializeProvisioner(p)
		assert.False(t, ok)

		_, ok = isResetProvisioner(p)
		assert.False(t, ok)
	}

	{ // initialize
		p = BuildProvisioner().
			WithInitialize(func(_ context.Context, _ StatusItem) error {
				return nil
			}).Create()

		_, ok = isLiveCheckProvisioner(p)
		assert.False(t, ok)

		_, ok = isInitializeProvisioner(p)
		assert.True(t, ok)

		_, ok = isResetProvisioner(p)
		assert.False(t, ok)
	}

	{ // reset
		p = BuildProvisioner().
			WithReset(func(_ context.Context, _ StatusItem) error {
				return nil
			}).Create()

		_, ok = isLiveCheckProvisioner(p)
		assert.False(t, ok)

		_, ok = isInitializeProvisioner(p)
		assert.False(t, ok)

		_, ok = isResetProvisioner(p)
		assert.True(t, ok)
	}

}
