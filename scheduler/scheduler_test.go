package scheduler_test

import (
	"testing"
)

func Test_Scheduler_ResolveImage(t *testing.T) {
	// ctx := testutil.Context()
	// node := testutil.Node(t, ctx)
	// bus := testutil.Bus(t, ctx)
	// sched := scheduler.New(ctx, bus, node)

	// ref, err := s.ResolveImage(context.Background(), "nginx:latest")
	// assert.NoError(t, err)
	// assert.Equal(t, "docker.io/library/nginx", ref.Name())

	// t.Logf("%#v\n", obj)

	// t.Log(obj.Name())
	// nginx        -> docker.io/library/nginx
	// nginx:latest -> docker.io/library/nginx

	// t.Log(obj.String())
	// nginx -> docker.io/library/nginx@sha256:cd5239a0906a6ccf0562354852fae04bc5b52d72a2aff9a871ddb6bd57553569
	// nginx:latest -> docker.io/library/nginx:latest@sha256:cd5239a0906a6ccf0562354852fae04bc5b52d72a2aff9a871ddb6bd57553569

	// t.Log(reference.FamiliarName(obj))
	// nginx        -> nginx
	// nginx:latest -> nginx

	// t.Log(reference.FamiliarString(obj))
	// nginx        -> nginx@sha256:cd5239a0906a6ccf0562354852fae04bc5b52d72a2aff9a871ddb6bd57553569
	// nginx:latest -> nginx:latest@sha256:cd5239a0906a6ccf0562354852fae04bc5b52d72a2aff9a871ddb6bd57553569
}
