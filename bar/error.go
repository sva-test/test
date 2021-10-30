package worker_error

import (
	"encoding/json"
	"fmt"
)

type err struct {
	Err   error  `json:"err,omitempty"`
	Extra string `json:"extra,omitempty"`
}

func New(e error, extra string) error {
	return err{Err: e, Extra: extra}
}

func (e err) Error() string {
	if d, er := json.Marshal(e); er == nil {
		return string(d)
	}

	return fmt.Sprintf("Err: %s | Extra: %s", e.Err, e.Extra)
}

func (e err) Unwrap() error {
	return e.Err
}
