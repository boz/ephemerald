package ui

import (
	"context"
	"encoding/json"
	"io"

	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/go-lifecycle"
	"github.com/sirupsen/logrus"
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
		l:   log.FromContext(ctx).WithField("cmp", "ui/event"),
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
	l   logrus.FieldLogger
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
			bytes, err := json.Marshal(ev)

			if err != nil {
				ui.l.WithError(err).Error("marshal event")
				continue
			}

			if _, err := ui.out.Write(bytes); err != nil {
				ui.l.WithError(err).Error("write output")
			}

			if _, err := ui.out.Write([]byte{'\n'}); err != nil {
				ui.l.WithError(err).Error("write newline")
			}

		}
	}
}
