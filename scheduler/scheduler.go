package scheduler

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
)

type Scheduler interface {
	ResolveImage(context.Context, string) (reference.Canonical, error)
}

func New() Scheduler {
	return &scheduler{}
}

type scheduler struct {
	dc *client.Client
}

func (s *scheduler) ResolveImage(ctx context.Context, name string) (reference.Canonical, error) {

	var (
		ii   types.ImageInspect
		err  error
		body io.ReadCloser
	)

	dc, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

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
		return nil, nil
	}
	defer body.Close()

	if _, err := ioutil.ReadAll(body); err != nil {
		return nil, err
	}

	ii, _, err = dc.ImageInspectWithRaw(ctx, ref.String())
	if err != nil {
		return nil, err
	}

done:
	digest, err := digest.Parse(ii.ID)
	if err != nil {
		return nil, err
	}
	// return reference.WithDigest(ref, digest.Digest(ii.ID))
	return reference.WithDigest(ref, digest)
}
