package ephemerald

import "github.com/sirupsen/logrus"

func lcid(log logrus.FieldLogger, id string) logrus.FieldLogger {
	return log.WithField("container", id[0:12])
}
