package secret

type errUnmanaged struct{}

func (errUnmanaged) GraphError() string {
	return "The secret is not managed by Console, unable to modify."
}

func (errUnmanaged) Error() string {
	return "unmanaged secret"
}

var ErrUnmanaged = errUnmanaged{}

type errAlreadyExists struct{}

func (errAlreadyExists) GraphError() string {
	return "The secret already exists, unable to create."
}

func (errAlreadyExists) Error() string {
	return "secret already exists"
}

var ErrAlreadyExists = errAlreadyExists{}
