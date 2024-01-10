package logger

import (
	"fmt"
	"strings"

	"github.com/bombsimon/logrusr/v4"
	"github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
)

// New creates a new logger with the given format and level
func New(format, level string) (logrus.FieldLogger, error) {
	log := logrus.StandardLogger()

	switch strings.ToLower(format) {
	case "json":
		log.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	default:
		return nil, fmt.Errorf("invalid log format: %q", format)
	}

	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, err
	}

	log.SetLevel(parsedLevel)

	// set an internal logger for klog (used by k8s client-go)
	klogLogger := logrus.New()
	klogLogger.SetLevel(logrus.WarnLevel)
	klogLogger.SetFormatter(log.Formatter)
	klog.SetLogger(logrusr.New(klogLogger))

	return log, nil
}
