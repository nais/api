package logger

import (
	"fmt"
	"strings"

	"github.com/bombsimon/logrusr/v4"
	"github.com/nais/api/internal/config"
	"github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
)

// New creates a new logger with the given format and level
func New(cfg config.Logger) (logrus.FieldLogger, error) {
	log := logrus.StandardLogger()

	switch strings.ToLower(cfg.Format) {
	case "json":
		log.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	default:
		return nil, fmt.Errorf("invalid log format: %q", cfg.Format)
	}

	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	log.SetLevel(level)

	// set an internal logger for klog (used by k8s client-go)
	klogLogger := logrus.New()
	klogLogger.SetLevel(logrus.WarnLevel)
	klogLogger.SetFormatter(log.Formatter)
	klog.SetLogger(logrusr.New(klogLogger))

	return log, nil
}
