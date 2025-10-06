package dr

import "fmt"

type Mode int

const (
	Active Mode = iota
	Standby
	Disabled
)

func (d Mode) IsActive() bool {
	return d == Active
}

func (d Mode) String() string {
	switch d {
	case Active:
		return "active"
	case Standby:
		return "standby"
	case Disabled:
		return "disabled"
	}
	return "unknown"
}

func ModeFromString(mode string) Mode {
	switch mode {
	case "active":
		return Active
	case "standby":
		return Standby
	case "disabled":
		return Disabled
	}
	panic(fmt.Sprintf("unknown mode: %s", mode))
}
