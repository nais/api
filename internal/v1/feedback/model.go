package feedback

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nais/api/internal/auth/authz"
)

type ReportConsoleUserFeedbackInput struct {
	Feedback  string                  `json:"feedback"`
	Path      string                  `json:"path"`
	Anonymous bool                    `json:"anonymous"`
	Type      ConsoleUserFeedbackType `json:"type"`
	Author    string                  `json:"-"`
}

func (input *ReportConsoleUserFeedbackInput) Sanitized(actor *authz.Actor) *ReportConsoleUserFeedbackInput {
	path := strings.TrimSpace(input.Path)
	if path == "" {
		path = "/"
	}

	author := "anonymous"
	if !input.Anonymous {
		author = "unknown"
		if actor != nil && actor.User != nil {
			author = actor.User.Identity()
		}
	}

	return &ReportConsoleUserFeedbackInput{
		Feedback:  strings.TrimSpace(input.Feedback),
		Path:      path,
		Anonymous: input.Anonymous,
		Type:      input.Type,
		Author:    author,
	}
}

type ReportConsoleUserFeedbackPayload struct {
	Reported *bool `json:"reported"`
}

type ConsoleUserFeedbackType string

const (
	ConsoleUserFeedbackTypeBug           ConsoleUserFeedbackType = "BUG"
	ConsoleUserFeedbackTypeChangeRequest ConsoleUserFeedbackType = "CHANGE_REQUEST"
	ConsoleUserFeedbackTypeOther         ConsoleUserFeedbackType = "OTHER"
	ConsoleUserFeedbackTypeQuestion      ConsoleUserFeedbackType = "QUESTION"
)

func (e ConsoleUserFeedbackType) IsValid() bool {
	switch e {
	case ConsoleUserFeedbackTypeBug, ConsoleUserFeedbackTypeChangeRequest, ConsoleUserFeedbackTypeOther, ConsoleUserFeedbackTypeQuestion:
		return true
	}
	return false
}

func (e ConsoleUserFeedbackType) String() string {
	return string(e)
}

func (e *ConsoleUserFeedbackType) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ConsoleUserFeedbackType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ConsoleUserFeedbackType", str)
	}
	return nil
}

func (e ConsoleUserFeedbackType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
