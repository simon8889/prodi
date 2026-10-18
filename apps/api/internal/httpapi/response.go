package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"prodi/api/internal/domain"
)

const maxBodyBytes = 1 << 20

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

func writeFieldError(w http.ResponseWriter, status int, code, message, field string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message, Field: field}})
}

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return domain.NewValidationError("body", "a JSON body is required")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return domain.NewValidationError("body", "the request body is not valid JSON")
		}
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return domain.NewValidationError(typeErr.Field, "has an unexpected type")
		}
		if errors.Is(err, io.EOF) {
			return domain.NewValidationError("body", "a JSON body is required")
		}
		return domain.NewValidationError("body", "the request body could not be decoded: "+err.Error())
	}
	return nil
}
