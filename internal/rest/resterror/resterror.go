package resterror

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type RestError struct {
	Status  int    `json:"status"`
	Message string `json:"errorMessage"`
}

func Wrap(status int, err error) *RestError {
	return &RestError{
		Status:  status,
		Message: err.Error(),
	}
}

func (r *RestError) ToJSON() string {
	bytes, _ := json.Marshal(r)
	return string(bytes)
}

func (r *RestError) Write(w http.ResponseWriter) {
	h := w.Header()

	// Delete the Content-Length header, which might be for some other content.
	// Assuming the error string fits in the writer's buffer, we'll figure
	// out the correct Content-Length for it later.
	//
	// We don't delete Content-Encoding, because some middleware sets
	// Content-Encoding: gzip and wraps the ResponseWriter to compress on-the-fly.
	// See https://go.dev/issue/66343.
	h.Del("Content-Length")

	// There might be content type already set, but we reset it to
	// application/json for the error message.
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(r.Status)
	fmt.Fprintln(w, r.ToJSON())
}
