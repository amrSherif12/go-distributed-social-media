package errs

import (
	"encoding/json"
	"errors"
)

type ErrType int

const (
	ValidationFailed = iota
	NotFound
	AuthEmailAlreadyRegistered
	AuthUnregisteredEmail
	AuthWrongPassword
)

type Err struct {
	Err     ErrType
	Message any
}

func (e *Err) Error() string {
	switch val := e.Message.(type) {
	case string:
		return val
	case map[string]string:
		jsonData, _ := json.MarshalIndent(val, "", "    ")
		return string(jsonData)
	default:
		panic("unexpected type while converting error")
	}
}

func Is(err error, errType ErrType) bool {
	var target *Err
	if errors.As(err, &target) {
		return target.Err == errType
	}
	return false
}

func IsErr(err error) *Err {
	var target *Err
	if errors.As(err, &target) {
		return target
	}
	return nil
}
