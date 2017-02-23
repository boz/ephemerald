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

type initializeProvisioner struct {
	initialize ProvisionFn
}

func (p *initializeProvisioner) Initialize(ctx context.Context, si StatusItem) error {
	return p.initialize(ctx, si)
}

type resetProvisioner struct {
	reset ProvisionFn
}

func (p *resetProvisioner) Reset(ctx context.Context, si StatusItem) error {
	return p.reset(ctx, si)
}

type allProvisioner struct {
	initializeProvisioner
	resetProvisioner
}

func (pb *provisionerBuilder) Create() Provisioner {
	switch {
	case pb.initialize != nil && pb.reset != nil:
		return &allProvisioner{initializeProvisioner{pb.initialize}, resetProvisioner{pb.reset}}
	case pb.initialize != nil:
		return &initializeProvisioner{pb.initialize}
	case pb.reset != nil:
		return &resetProvisioner{pb.reset}
	}
	return struct{}{}
}
