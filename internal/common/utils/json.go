package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"go_distributed_social_media/internal/common/errs"
	"io"
	"net/http"
)

func ParseJson(r *http.Request, obj any) error {
	if r.Body == nil {
		return errors.New("missing request body")
	}
	err := json.NewDecoder(r.Body).Decode(obj)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body cannot be empty")
		}
		return fmt.Errorf("invalid JSON payload: %w", err)
	}
	return nil
}

func WriteJson(w http.ResponseWriter, status int, obj any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(obj)
}

func WriteError(w http.ResponseWriter, status int, err error) error {
	if Err := errs.IsErr(err); Err != nil {
		return WriteJson(w, status, map[string]any{"error": Err.Message})
	}
	return WriteJson(w, status, map[string]string{"error": err.Error()})
}
