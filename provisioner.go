package ephemerald

import "context"

type Provisioner interface{}

type ProvisionFn func(context.Context, StatusItem) error

type provisionerWithLiveCheck interface {
	LiveCheck(context.Context, StatusItem) error
}

type provisionerWithInitialize interface {
	Initialize(context.Context, StatusItem) error
}

type provisionerWithReset interface {
	Reset(context.Context, StatusItem) error
}

func isLiveCheckProvisioner(h Provisioner) (provisionerWithLiveCheck, bool) {
	switch h := h.(type) {
	case *builtProvisioner:
		return h, h.livecheck != nil
	case provisionerWithLiveCheck:
		return h, true
	default:
		return nil, false
	}
}

func isInitializeProvisioner(h Provisioner) (provisionerWithInitialize, bool) {
	switch h := h.(type) {
	case *builtProvisioner:
		return h, h.initialize != nil
	case provisionerWithInitialize:
		return h, true
	default:
		return nil, false
	}
}

func isResetProvisioner(h Provisioner) (provisionerWithReset, bool) {
	switch h := h.(type) {
	case *builtProvisioner:
		return h, h.reset != nil
	case provisionerWithReset:
		return h, true
	default:
		return nil, false
	}
}

type ProvisionerBuilder interface {
	WithLiveCheck(ProvisionFn) ProvisionerBuilder
	WithInitialize(ProvisionFn) ProvisionerBuilder
	WithReset(ProvisionFn) ProvisionerBuilder
	Clone() ProvisionerBuilder
	Create() Provisioner
}

func BuildProvisioner() ProvisionerBuilder {
	return &provisionerBuilder{}
}

type provisionerBuilder struct {
	livecheck  ProvisionFn
	initialize ProvisionFn
	reset      ProvisionFn
}

func (pb *provisionerBuilder) WithLiveCheck(fn ProvisionFn) ProvisionerBuilder {
	pb.livecheck = fn
	return pb
}

func (pb *provisionerBuilder) WithInitialize(fn ProvisionFn) ProvisionerBuilder {
	pb.initialize = fn
	return pb
}

func (pb *provisionerBuilder) WithReset(fn ProvisionFn) ProvisionerBuilder {
	pb.reset = fn
	return pb
}

func (pb *provisionerBuilder) Clone() ProvisionerBuilder {
	return &(*pb)
}

func (pb *provisionerBuilder) Create() Provisioner {
	built := builtProvisioner(*pb)
	return &built
}

type builtProvisioner provisionerBuilder

func (p *builtProvisioner) LiveCheck(ctx context.Context, si StatusItem) error {
	if p.livecheck == nil {
		return nil
	}
	return p.livecheck(ctx, si)
}

func (p *builtProvisioner) Initialize(ctx context.Context, si StatusItem) error {
	if p.initialize == nil {
		return nil
	}
	return p.initialize(ctx, si)
}

func (p *builtProvisioner) Reset(ctx context.Context, si StatusItem) error {
	if p.reset == nil {
		return nil
	}
	return p.reset(ctx, si)
}
