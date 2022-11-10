package controllers

type SleepCycleOperation int

const (
	Watch SleepCycleOperation = iota
	Shutdown
	WakeUp
)

func (sco SleepCycleOperation) String() string {
	var values []string = []string{"Watch", "Shutdown", "Wakeup"}
	name := values[sco]

	return name
}
