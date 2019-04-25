package log

import (
	"context"

	"github.com/sirupsen/logrus"
)

var (
	logger = logrus.New()
)

func Default() *logrus.Logger {
	return logger
}

var ctxKey = struct{}{}

func NewContext(ctx context.Context, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, ctxKey, log)
}

func FromContext(ctx context.Context) logrus.FieldLogger {
	val := ctx.Value(ctxKey)
	if log, ok := val.(logrus.FieldLogger); ok {
		return log
	}
	return Default()
}
