package watcher

import (
	"fmt"
	"strings"
)

type ErrorNotFound struct {
	Cluster   string
	Namespace string
	Name      string
}

func (e *ErrorNotFound) Error() string {
	return "not found: " + e.Cluster + "/" + e.Namespace + "/" + e.Name
}

func (e *ErrorNotFound) GraphError() string {
	return "Resource not found: " + e.Cluster + "/" + e.Namespace + "/" + e.Name
}

func (e *ErrorNotFound) As(v any) bool {
	if _, ok := v.(*ErrorNotFound); ok {
		return true
	}

	return false
}

func (e *ErrorNotFound) Is(v error) bool {
	if _, ok := v.(*ErrorNotFound); ok {
		return true
	}

	return false
}

type ErrorUnknownEnvironment struct {
	Environment string
	Valid       []string
}

func (e *ErrorUnknownEnvironment) Error() string {
	return "unknown environment: " + e.Environment
}

func (e *ErrorUnknownEnvironment) GraphError() string {
	return fmt.Sprintf("Unknown environment %q. Valid values are [%s]", e.Environment, strings.Join(e.Valid, ", "))
}

func (e *ErrorUnknownEnvironment) As(v any) bool {
	if _, ok := v.(*ErrorUnknownEnvironment); ok {
		return true
	}

	return false
}

func (e *ErrorUnknownEnvironment) Is(v error) bool {
	if _, ok := v.(*ErrorUnknownEnvironment); ok {
		return true
	}

	return false
}
