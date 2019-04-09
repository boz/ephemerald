package scheduler

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/boz/ephemerald/node"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
)

type Scheduler interface {
	ResolveImage(context.Context, string) (reference.Canonical, error)
}

func New(node node.Node) Scheduler {
	return &scheduler{
		node: node,
	}
}

type scheduler struct {
	node node.Node
}

func (s *scheduler) ResolveImage(ctx context.Context, name string) (reference.Canonical, error) {

	var (
		ii   types.ImageInspect
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

	body, err = dc.ImagePull(ctx, ref.String(), types.ImagePullOptions{})
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
