// Package api implements the gateway's public REST surface: request
// validation, auth/CORS/rate-limit middleware, handlers, and the router.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// errorResponse is the uniform error envelope: {"error":{"code","message"}}.
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respondJSON(w http.ResponseWriter, log *slog.Logger, status int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		log.Error("marshal response", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(buf); err != nil {
		log.Warn("write response", "error", err)
	}
}

func respondError(w http.ResponseWriter, log *slog.Logger, status int, code, message string) {
	respondJSON(w, log, status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

// requestError carries a client-facing decode/validation failure.
type requestError struct {
	code    string
	message string
}

func (e *requestError) Error() string { return e.message }

func badRequest(format string, args ...any) *requestError {
	return &requestError{code: "bad_request", message: fmt.Sprintf(format, args...)}
}

// decodeJSON parses a JSON body with a size cap and strict field checking.
func decodeJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return &requestError{code: "payload_too_large", message: fmt.Sprintf("request body exceeds %d bytes", maxErr.Limit)}
		}
		return badRequest("invalid JSON body: %v", err)
	}
	if dec.More() {
		return badRequest("request body must contain a single JSON object")
	}
	return nil
}

// respondRequestError writes a requestError with the right status.
func respondRequestError(w http.ResponseWriter, log *slog.Logger, err *requestError) {
	status := http.StatusBadRequest
	if err.code == "payload_too_large" {
		status = http.StatusRequestEntityTooLarge
	}
	respondError(w, log, status, err.code, err.message)
}
