package botfile

// XXX
type Flow interface {
	Setup() error

	Start() error
	Stop() error
}
