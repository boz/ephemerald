package cpool

import "context"

type Provisioner interface{}

type ProvisionFn func(context.Context, StatusItem) error

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

type ProvisionerBuilder interface {
	WithInitialize(ProvisionFn) ProvisionerBuilder
	WithReset(ProvisionFn) ProvisionerBuilder
	Create() Provisioner
}

func BuildProvisioner() ProvisionerBuilder {
	return &provisionerBuilder{}
}

type provisionerBuilder struct {
	initialize ProvisionFn
	reset      ProvisionFn
}

func (pb *provisionerBuilder) WithInitialize(fn ProvisionFn) ProvisionerBuilder {
	pb.initialize = fn
	return pb
}

func (pb *provisionerBuilder) WithReset(fn ProvisionFn) ProvisionerBuilder {
	pb.reset = fn
	return pb
}

func (pb *provisionerBuilder) Create() Provisioner {
	switch {
	case pb.initialize != nil && pb.reset != nil:
		return struct {
			Initialize ProvisionFn
			Reset      ProvisionFn
		}{pb.initialize, pb.reset}
	case pb.initialize != nil:
		return struct {
			Initialize ProvisionFn
		}{pb.initialize}
	case pb.reset != nil:
		return struct {
			Reset ProvisionFn
		}{pb.reset}
	}
	return struct{}{}
}
