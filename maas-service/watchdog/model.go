package watchdog

type Status string

const (
	UNKNOWN Status = "UNKNOWN"
	UP      Status = "UP"
	WARNING Status = "WARNING"
	PROBLEM Status = "PROBLEM"
)

func (hs Status) Int() int {
	return map[Status]int{
		UNKNOWN: 0,
		UP:      10,
		WARNING: 20,
		PROBLEM: 30,
	}[hs]
}

type StatusDetailed struct {
	Status  Status `json:"status"`
	Details string `json:"details,omitempty"`
}
