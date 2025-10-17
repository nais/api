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
	return "A secret with this name already exists."
}

func (errAlreadyExists) Error() string {
	return "secret already exists"
}

var ErrAlreadyExists = errAlreadyExists{}
