package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/ui"
)

var (
	ErrRetryCountExceeded = fmt.Errorf("retry count exceeded")
)

type actionRunner struct {
	action     Action
	actionType string
	actionName string
	p          params.Params
	ctx        context.Context
	log        logrus.FieldLogger
	emitter    ui.ContainerEmitter
}

func newActionRunner(ctx context.Context, emitter ui.ContainerEmitter, log logrus.FieldLogger, action Action, p params.Params, actionName string) *actionRunner {

	actionType := action.Config().Type

	log = log.WithField("action", actionName).
		WithField("type", actionType)

	return &actionRunner{
		action:     action,
		actionType: actionType,
		actionName: actionName,
		p:          p,
		ctx:        ctx,
		log:        log,
		emitter:    emitter,
	}
}

func (ar *actionRunner) Run() error {

	attempt := 1
	retries := ar.action.Config().Retries
	timeout := ar.action.Config().Timeout
	delay := ar.action.Config().Delay

	maxAttempts := retries + 1

	for {

		if ar.ctx.Err() != nil {
			ar.emitter.EmitActionResult(ar.actionName, ar.actionType, attempt, maxAttempts, ar.ctx.Err())
			return ar.ctx.Err()
		}

		ar.emitter.EmitActionAttempt(ar.actionName, ar.actionType, attempt, maxAttempts)

		err, ok := ar.doAttempt(attempt, timeout)

		ar.emitter.EmitActionResult(ar.actionName, ar.actionType, attempt, maxAttempts, err)

		if !ok {
			return err
		}

		if err == nil {
			return nil
		}

		if attempt > retries {
			ar.log.WithError(err).Warn("retry count exceeded")
			return ErrRetryCountExceeded
		}

		attempt++

		select {
		case <-ar.ctx.Done():
			return ar.ctx.Err()
		case <-time.After(delay):
			// retry
		}
	}
}

func (ar *actionRunner) doAttempt(attempt int, timeout time.Duration) (error, bool) {
	ch := make(chan error)
	defer close(ch)

	ctx, cancel := context.WithTimeout(ar.ctx, timeout)
	defer cancel()

	env := NewEnv(ctx, ar.log.WithField("attempt", attempt))

	go func() {
		err := ar.action.Do(env, ar.p)
		if ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case ch <- err:
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err(), ar.ctx.Err() == nil
	case err := <-ch:
		return err, true
	}
}
