package secret

type errUnmanagedSecret struct{}

func (errUnmanagedSecret) GraphError() string {
	return "The secret is not managed by Console, unable to modify."
}

func (errUnmanagedSecret) Error() string {
	return "unmanaged secret"
}

var ErrUnmanagedSecret = errUnmanagedSecret{}
