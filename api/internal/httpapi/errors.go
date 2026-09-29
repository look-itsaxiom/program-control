package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Error codes this API answers with. They are part of the contract: a client switches on these
// rather than on prose or on the status code alone.
const (
	codeUnauthenticated = "unauthenticated"
	codeNotFound        = "not_found"
	codeValidation      = "validation"
	codeDuplicate       = "duplicate"
	codeDuplicateTitle  = "duplicate_title"
	codeCycle           = "cycle"
	codeMethod          = "method_not_allowed"
	codeInternal        = "internal"
)

// fieldError names one field a request got wrong and says what is wrong with it.
type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type errorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []fieldError `json:"details,omitempty"`
}

// errorEnvelope is the shape of every error this API returns, from every route.
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// writeJSON writes v as the response body with the given status. Encoding failures can only
// happen after the header is out, so they are logged rather than turned into a second response.
func writeJSON(w http.ResponseWriter, log *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error("write json response", "error", err)
	}
}

// writeError answers with the shared error envelope.
func writeError(w http.ResponseWriter, log *slog.Logger, status int, code, message string) {
	writeJSON(w, log, status, errorEnvelope{Error: errorBody{Code: code, Message: message}})
}

// writeValidationError answers 400 with one details entry per field the request got wrong.
func writeValidationError(w http.ResponseWriter, log *slog.Logger, details []fieldError) {
	writeJSON(w, log, http.StatusBadRequest, errorEnvelope{Error: errorBody{
		Code:    codeValidation,
		Message: "the request body is not valid",
		Details: details,
	}})
}
