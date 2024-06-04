package controllers

type OpCode int

const (
	Terminate OpCode = iota - 1
	Shutdown
	Wakeup
)

func (o OpCode) String() string {
	suffix := "shutdown"
	switch o {
	case Terminate:
		suffix = "terminate"
	case Wakeup:
		suffix = "wakeup"
	case Shutdown:
		suffix = "shutdown"
	}

	return suffix
}
