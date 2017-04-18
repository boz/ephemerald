package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/params"
)

var (
	ErrRetryCountExceeded = fmt.Errorf("retry count exceeded")
)

type actionRunner struct {
	action  Action
	p       params.Params
	ctx     context.Context
	log     logrus.FieldLogger
	attempt int
}

func newActionRunner(ctx context.Context, log logrus.FieldLogger, action Action, p params.Params, actionName string) *actionRunner {
	log = log.WithField("action", actionName).
		WithField("type", action.Config().Type)

	return &actionRunner{
		action: action,
		p:      p,
		ctx:    ctx,
		log:    log,
	}
}

func (ar *actionRunner) Run() error {

	attempt := 0
	retries := ar.action.Config().Retries
	timeout := ar.action.Config().Timeout
	delay := ar.action.Config().Delay

	for {

		if ar.ctx.Err() != nil {
			return ar.ctx.Err()
		}

		err, ok := ar.doAttempt(attempt, timeout)
		if !ok {
			return err
		}

		if err == nil {
			return nil
		}

		attempt++

		if attempt >= retries {
			ar.log.WithError(err).Warn("retry count exceeded")
			return ErrRetryCountExceeded
		}

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
