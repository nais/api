package tunnel

import "errors"

var (
	ErrTunnelNotFound = errors.New("tunnel not found")
	ErrTunnelNotReady = errors.New("tunnel not ready")
	ErrNotImplemented = errors.New("not implemented")
)
