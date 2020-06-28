package zebou

type ConnectionStateHandler interface {
	ConnectionState(active bool)
}

type ConnectionStateHandlerFunc func(bool)

func (f ConnectionStateHandlerFunc) ConnectionState(active bool) {
	f(active)
}
