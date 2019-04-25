package ui

import (
	"context"
	"fmt"
	"io"

	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/go-lifecycle"
)

func NewEVLog(ctx context.Context, bus pubsub.Reader, out io.Writer) (UI, error) {

	sub, err := bus.Subscribe(pubsub.FilterNone)
	if err != nil {
		return nil, err
	}

	ui := &evlog{
		ctx: ctx,
		sub: sub,
		out: out,
		lc:  lifecycle.New(),
	}

	go ui.lc.WatchContext(ctx)
	go ui.run()

	return ui, nil
}

type evlog struct {
	ctx context.Context
	sub pubsub.Subscription
	out io.Writer
	lc  lifecycle.Lifecycle
}

func (ui *evlog) Stop() {
	ui.lc.Shutdown(nil)
}

func (ui *evlog) run() {
	defer ui.lc.ShutdownCompleted()

loop:
	for {
		select {
		case err := <-ui.lc.ShutdownRequest():
			ui.lc.ShutdownInitiated(err)
			break loop
		case ev := <-ui.sub.Events():
			fmt.Fprintf(ui.out, "%#v\n", ev)
		}
	}
}
