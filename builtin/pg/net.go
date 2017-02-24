package pg

import (
	"context"

	"github.com/koding/kite"
)

type NetPool struct {
	pool Pool
}

func BuildForRequest(r *kite.Request) (*NetPool, error) {
	pool, err := DefaultBuilder().
		WithInitialize(func(ctx context.Context, i Item) error {
			return RemoteDo(ctx, r, "pg.initialize", i)
		}).
		WithReset(func(ctx context.Context, i Item) error {
			return RemoteDo(ctx, r, "pg.reset", i)
		}).Create()

	if err != nil {
		return nil, err
	}

	return &NetPool{pool}, err
}

func RemoteDo(ctx context.Context, r *kite.Request, name string, i interface{}) error {
	ch := r.Client.Go(name, i)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-ch:
		return resp.Err
	}
}
