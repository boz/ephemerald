package cpool

import "context"

type Provisioner interface{}

type provisionerWithReset interface {
	Reset(context.Context, StatusItem) error
}

type provisionerWithIntitialize interface {
	Initialize(context.Context, StatusItem) error
}

func isResetProvisioner(h Provisioner) (provisionerWithReset, bool) {
	h2, ok := h.(provisionerWithReset)
	return h2, ok
}

func isInitializeProvisioner(h Provisioner) (provisionerWithIntitialize, bool) {
	h2, ok := h.(provisionerWithIntitialize)
	return h2, ok
}
