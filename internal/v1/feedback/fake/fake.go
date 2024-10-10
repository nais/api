package fakefeedback

import (
	"context"

	"github.com/nais/api/internal/v1/feedback"
	"github.com/sirupsen/logrus"
)

type fake struct {
	log logrus.FieldLogger
}

func NewClient(log logrus.FieldLogger) feedback.Client {
	return &fake{
		log: log,
	}
}

func (c *fake) PostFeedback(_ context.Context, input *feedback.ReportConsoleUserFeedbackInput) error {
	c.log.WithFields(logrus.Fields{
		"feedback":  input.Feedback,
		"path":      input.Path,
		"type":      input.Type,
		"anonymous": input.Anonymous,
		"author":    input.Author,
	}).Debugf("posting feedback")
	return nil
}
