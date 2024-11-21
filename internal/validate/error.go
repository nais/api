package validate

import (
	"fmt"
	"strings"
)

type ValidationErrors struct {
	Errors []*ValidationError `json:"errors"`
}

func New() *ValidationErrors {
	return &ValidationErrors{
		Errors: make([]*ValidationError, 0),
	}
}

func (e *ValidationErrors) Error() string {
	ret := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		ret[i] = err.String()
	}
	return strings.Join(ret, "\n")
}

func (e *ValidationErrors) Add(field, message string, args ...any) {
	e.Errors = append(e.Errors, &ValidationError{
		GraphQLField: &field,
		Message:      fmt.Sprintf(message, args...),
	})
}

func (e *ValidationErrors) AddMessage(message string, args ...any) {
	e.Errors = append(e.Errors, &ValidationError{
		Message: fmt.Sprintf(message, args...),
	})
}

func (e *ValidationErrors) NilIfEmpty() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e
}

type ValidationError struct {
	GraphQLField *string `json:"field"`
	Message      string  `json:"message"`
}

func (e ValidationError) String() string {
	if e.GraphQLField == nil {
		return e.Message
	}
	return *e.GraphQLField + ": " + e.Message
}
