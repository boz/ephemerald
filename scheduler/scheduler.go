package scheduler

import (
	"context"

	"github.com/docker/docker/api/types"
)

type Scheduler interface {
	ResolveImage(context.Context, string) (types.ImageInspect, error)
}

type scheduler struct {
}

func (s *scheduler) ResolveImage(ctx context.Context, name string) (types.ImageInspect, error) {
	// ImageInspectWithRaw

	// if not present

	// ImageCreate
	return types.ImageInspect{}, nil
}
