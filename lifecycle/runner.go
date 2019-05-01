package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/types"
	"github.com/sirupsen/logrus"
)

var (
	ErrRetryCountExceeded = fmt.Errorf("retry count exceeded")
)

type actionRunner struct {
	bus      pubsub.Bus
	instance types.Instance
	action   Action
	model    *types.LifecycleAction
	p        params.Params
	ctx      context.Context
	log      logrus.FieldLogger
}

func newActionRunner(bus pubsub.Bus, instance types.Instance, ctx context.Context, action Action, p params.Params, actionName string) *actionRunner {

	actionType := action.Config().Type

	log := log.FromContext(ctx)
	log = log.WithField("action", actionName).
		WithField("type", actionType)

	log.Infof("running action %#v", action)

	return &actionRunner{
		bus:    bus,
		action: action,
		model: &types.LifecycleAction{
			Instance:   instance,
			State:      types.LifecycleActionStateRunning,
			Name:       actionName,
			Type:       actionType,
			MaxRetries: uint(action.Config().Retries),
		},
		p:   p,
		ctx: ctx,
		log: log,
	}
}

func (ar *actionRunner) Run() error {

	timeout := ar.action.Config().Timeout
	delay := ar.action.Config().Delay

	var (
		err error
		ok  bool
	)

	for {

		if ar.ctx.Err() != nil {
			return ar.publishResult(ar.ctx.Err())
		}

		ar.bus.Publish(types.Event{
			Type:            types.EventTypeLifecycleAction,
			Action:          types.EventActionStart,
			LifecycleAction: &(*ar.model),
			Status:          types.StatusInProgress,
		})

		err, ok = ar.doAttempt(timeout)

		if !ok || err == nil {
			return ar.publishResult(err)
		}

		if ar.model.Retries >= ar.model.MaxRetries {
			ar.log.WithError(err).Warn("retry count exceeded")
			return ar.publishResult(err)
		}

		ar.model.State = types.LifecycleActionStateRetryWait

		ar.bus.Publish(types.Event{
			Type:            types.EventTypeLifecycleAction,
			Action:          types.EventActionAttemptFailed,
			LifecycleAction: &(*ar.model),
			Status:          types.StatusInProgress,
			Message:         err.Error(),
		})

		select {
		case <-ar.ctx.Done():
			return ar.publishResult(ar.ctx.Err())
		case <-time.After(delay):
			ar.model.Retries++
		}
	}
}

func (ar *actionRunner) publishResult(err error) error {

	ar.model.State = types.LifecycleActionStateDone

	ev := types.Event{
		Type:            types.EventTypeLifecycleAction,
		Action:          types.EventActionDone,
		LifecycleAction: &(*ar.model),
	}

	if err == nil {
		ev.Status = types.StatusSuccess
	} else {
		ev.Status = types.StatusFailure
		ev.Message = err.Error()
	}

	ar.bus.Publish(ev)

	return err
}

func (ar *actionRunner) doAttempt(timeout time.Duration) (error, bool) {
	ch := make(chan error)

	ctx, cancel := context.WithTimeout(ar.ctx, timeout)
	defer cancel()

	env := NewEnv(ctx, ar.log.WithField("attempt", ar.model.Retries+1))

	go func() {
		err := ar.action.Do(env, ar.p)
		select {
		case <-ctx.Done():
		case ch <- err:
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err(), ar.ctx.Err() == nil
	case err := <-ch:
		return err, true
	}
}
