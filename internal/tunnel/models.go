package tunnel

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

type TunnelTarget struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type TunnelPhase string

const (
	TunnelPhasePending      TunnelPhase = "PENDING"
	TunnelPhaseProvisioning TunnelPhase = "PROVISIONING"
	TunnelPhaseReady        TunnelPhase = "READY"
	TunnelPhaseConnected    TunnelPhase = "CONNECTED"
	TunnelPhaseFailed       TunnelPhase = "FAILED"
	TunnelPhaseTerminated   TunnelPhase = "TERMINATED"
)

var AllTunnelPhase = []TunnelPhase{
	TunnelPhasePending,
	TunnelPhaseProvisioning,
	TunnelPhaseReady,
	TunnelPhaseConnected,
	TunnelPhaseFailed,
	TunnelPhaseTerminated,
}

func (e TunnelPhase) IsValid() bool {
	switch e {
	case TunnelPhasePending, TunnelPhaseProvisioning, TunnelPhaseReady, TunnelPhaseConnected, TunnelPhaseFailed, TunnelPhaseTerminated:
		return true
	}
	return false
}

func (e TunnelPhase) String() string {
	return string(e)
}

func (e *TunnelPhase) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = TunnelPhase(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid TunnelPhase", str)
	}
	return nil
}

func (e TunnelPhase) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func (e *TunnelPhase) UnmarshalJSON(b []byte) error {
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	return e.UnmarshalGQL(s)
}

func (e TunnelPhase) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	e.MarshalGQL(&buf)
	return buf.Bytes(), nil
}
