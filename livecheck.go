package cpool

import (
	"context"
	"fmt"
	"time"
)

const (
	LiveCheckDefaultTimeout = time.Second
	LiveCheckDefaultRetries = 10
	LiveCheckDefaultDelay   = time.Second
)

var RetryCountExceeded = fmt.Errorf("retry count exceeded")

func LiveCheck(timeout time.Duration, tries int, delay time.Duration, fn ProvisionFn) ProvisionFn {
	return LiveCheckRetry(tries, delay, LiveCheckTimeout(timeout, fn))
}

func LiveCheckTimeout(timeout time.Duration, fn ProvisionFn) ProvisionFn {
	return func(ctx context.Context, si StatusItem) error {

		checkctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		errch := make(chan error)

		go func() {
			defer close(errch)
			select {
			case errch <- fn(checkctx, si):
			}
		}()

		select {
		case <-checkctx.Done():
			return checkctx.Err()
		case err := <-errch:

			return err
		}
	}
}

func LiveCheckRetry(tries int, delay time.Duration, fn ProvisionFn) ProvisionFn {
	return func(ctx context.Context, si StatusItem) error {

		for i := 0; i < tries; i++ {

			if ctx.Err() != nil {
				return ctx.Err()
			}

			errch := make(chan error)

			go func() {
				defer close(errch)
				select {
				case errch <- fn(ctx, si):
				}
			}()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-errch:
				if err == nil {
					return nil
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// retry
			}
		}

		return RetryCountExceeded
	}
}
