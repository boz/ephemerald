package ephemerald

import "github.com/Sirupsen/logrus"

func lcid(log logrus.FieldLogger, id string) logrus.FieldLogger {
	return log.WithField("container", id[0:12])
}
