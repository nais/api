package event

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type WorkloadSuccessfulRescaleEventDirection string

const (
	WorkloadSuccessfulRescaleEventDirectionUp   WorkloadSuccessfulRescaleEventDirection = "UP"
	WorkloadSuccessfulRescaleEventDirectionDown WorkloadSuccessfulRescaleEventDirection = "DOWN"
)

var AllWorkloadSuccessfulRescaleEventDirection = []WorkloadSuccessfulRescaleEventDirection{
	WorkloadSuccessfulRescaleEventDirectionUp,
	WorkloadSuccessfulRescaleEventDirectionDown,
}

func (e WorkloadSuccessfulRescaleEventDirection) IsValid() bool {
	return e == WorkloadSuccessfulRescaleEventDirectionUp || e == WorkloadSuccessfulRescaleEventDirectionDown
}

func (e WorkloadSuccessfulRescaleEventDirection) String() string {
	return string(e)
}

func (e *WorkloadSuccessfulRescaleEventDirection) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	direction := WorkloadSuccessfulRescaleEventDirection(str)
	if !direction.IsValid() {
		return fmt.Errorf("%s is not a valid WorkloadSuccessfulRescaleEventDirection", str)
	}
	*e = direction
	return nil
}

func (e WorkloadSuccessfulRescaleEventDirection) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func (e *WorkloadSuccessfulRescaleEventDirection) UnmarshalJSON(data []byte) error {
	var direction string
	if err := json.Unmarshal(data, &direction); err != nil {
		return err
	}
	// Stored Kubernetes events use lowercase directions.
	return e.UnmarshalGQL(strings.ToUpper(direction))
}
