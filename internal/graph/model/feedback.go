package model

import (
	"fmt"
	"io"
	"strconv"
)

type CreateFeedbackResult struct {
	// Whether the feedback was created or not.
	Created bool    `json:"created"`
	Error   *string `json:"error,omitempty"`
}

type CreateFeedbackInput struct {
	// The feedback content.
	Details   string       `json:"details"`
	URI       string       `json:"uri"`
	Anonymous bool         `json:"anonymous"`
	Type      FeedbackType `json:"type"`
}

type FeedbackType string

const (
	// Feedback type for the feedback.
	FeedbackTypeBug           FeedbackType = "BUG"
	FeedbackTypeChangeRequest FeedbackType = "CHANGE_REQUEST"
	FeedbackTypeOther         FeedbackType = "OTHER"
	FeedbackTypeQuestion      FeedbackType = "QUESTION"
)

var AllFeedbackType = []FeedbackType{
	FeedbackTypeBug,
	FeedbackTypeChangeRequest,
	FeedbackTypeOther,
	FeedbackTypeQuestion,
}

func (e FeedbackType) IsValid() bool {
	switch e {
	case FeedbackTypeBug, FeedbackTypeChangeRequest, FeedbackTypeOther, FeedbackTypeQuestion:
		return true
	}
	return false
}

func (e FeedbackType) String() string {
	return string(e)
}

func (e *FeedbackType) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = FeedbackType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid FeedbackType", str)
	}
	return nil
}

func (e FeedbackType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
