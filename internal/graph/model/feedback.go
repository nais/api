package model

type CreateFeedbackInput struct {
	// The feedback content.
	Content   string `json:"content"`
	URI       string `json:"uri"`
	Anonymous bool   `json:"anonymous"`
}

type CreateFeedbackResult struct {
	// Whether the feedback was created or not.
	Created bool    `json:"created"`
	Error   *string `json:"error,omitempty"`
}
