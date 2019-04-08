package lifecycle

import (
	"context"

	"github.com/sirupsen/logrus"
)

type Env interface {
	Context() context.Context
	Log() logrus.FieldLogger
}

type env struct {
	ctx context.Context
	log logrus.FieldLogger
}

func NewEnv(ctx context.Context, log logrus.FieldLogger) Env {
	return &env{ctx, log}
}

func (e *env) Context() context.Context {
	return e.ctx
}

func (e *env) Log() logrus.FieldLogger {
	return e.log
}
