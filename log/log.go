package log

import (
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	logger *logrus.Logger
	mtx    sync.Mutex
)

func New() logrus.FieldLogger {
	mtx.Lock()
	defer mtx.Unlock()
	return get()
}

func get() *logrus.Logger {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.DebugLevel)
	}
	return logger
}
