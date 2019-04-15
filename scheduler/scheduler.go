package scheduler

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/boz/ephemerald/instance"
	"github.com/boz/ephemerald/node"
	"github.com/boz/ephemerald/pubsub"
	"github.com/docker/distribution/reference"
	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
)

type Scheduler interface {
	ResolveImage(context.Context, string) (reference.Canonical, error)

	CreateInstance(context.Context, instance.Config) (instance.Instance, error)
}

func New(bus pubsub.Bus, node node.Node) Scheduler {
	return &scheduler{
		bus:  bus,
		node: node,
	}
}

type scheduler struct {
	bus  pubsub.Bus
	node node.Node
}

func (s *scheduler) ResolveImage(ctx context.Context, name string) (reference.Canonical, error) {

	var (
		ii   dtypes.ImageInspect
		err  error
		body io.ReadCloser
	)

	dc := s.node.Client()

	ref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return nil, err
	}

	ii, _, err = dc.ImageInspectWithRaw(ctx, ref.String())
	if err == nil {
		goto done
	}
	if !client.IsErrNotFound(err) {
		return nil, err
	}

	body, err = dc.ImagePull(ctx, ref.String(), dtypes.ImagePullOptions{})
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(ioutil.Discard, body); err != nil {
		body.Close()
		return nil, err
	}
	body.Close()

	ii, _, err = dc.ImageInspectWithRaw(ctx, ref.String())
	if err != nil {
		return nil, err
	}

done:
	digest, err := digest.Parse(ii.ID)
	if err != nil {
		return nil, err
	}
	return reference.WithDigest(ref, digest)
}

func (s *scheduler) CreateInstance(ctx context.Context, config instance.Config) (instance.Instance, error) {
	return instance.Create(s.bus, s.node, config)
}
