package configmap

type errUnmanaged struct{}

func (errUnmanaged) GraphError() string {
	return "The config is not managed by Console, unable to modify."
}

func (errUnmanaged) Error() string {
	return "unmanaged config"
}

var ErrUnmanaged = errUnmanaged{}

type errAlreadyExists struct{}

func (errAlreadyExists) GraphError() string {
	return "A config with this name already exists."
}

func (errAlreadyExists) Error() string {
	return "config already exists"
}

var ErrAlreadyExists = errAlreadyExists{}
