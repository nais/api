package test

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type RoundTripFunc func(req *http.Request) *http.Response

type RoundTripper struct {
	funcs          []RoundTripFunc
	requestCounter int
}

func (r *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	handler := r.funcs[r.requestCounter]
	r.requestCounter++
	return handler(req), nil
}

func NewTestHttpClient(funcs ...RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: &RoundTripper{
			funcs: funcs,
		},
	}
}

func Response(status string, body string) *http.Response {
	parts := strings.Fields(status)
	statusCode, _ := strconv.Atoi(parts[0])

	return &http.Response{
		Status:     status,
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}
