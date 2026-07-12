package messages

import (
	"encoding/json"
	"strconv"
)

type Uint64String uint64

func (u *Uint64String) UnmarshalJSON(data []byte) error {
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		v, err := strconv.ParseUint(string(n), 10, 64)
		if err != nil {
			return err
		}
		*u = Uint64String(v)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*u = 0
		return nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return err
	}
	*u = Uint64String(v)
	return nil
}

// MarshalJSON outputs the value as a JSON string.
func (u Uint64String) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(u), 10))
}

// Uint64 returns the underlying uint64 value.
func (u Uint64String) Uint64() uint64 {
	return uint64(u)
}

// String returns the string representation.
func (u Uint64String) String() string {
	return strconv.FormatUint(uint64(u), 10)
}

// FromUint64 creates a Uint64String from uint64.
func Uint64StringFromUint64(v uint64) Uint64String {
	return Uint64String(v)
}
