package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

const (
	headerTimestamp = "X-Timestamp"
	headerSignature = "X-Signature"

	// webhookSkew is how far a message's timestamp may be from this server's clock. A valid
	// signature on an old message is exactly what a captured and replayed request looks like,
	// so age is checked even when the signature is good.
	webhookSkew = 5 * time.Minute

	// maxWebhookBody bounds what an unauthenticated caller can make this process read.
	maxWebhookBody = 1 << 20
)

// webhookSecrets is the set of secrets a signature may match: the current one, and the previous
// one while a rotation is in flight.
type webhookSecrets struct {
	current  string
	previous string
}

type webhookMessage struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	TaskID string `json:"taskId"`
	Status string `json:"status"`
}

// receiveVendorWebhook accepts a status change from a vendor's system. There is no session here
// and no person: the shared secret is the whole of the trust, so the signature is checked before
// anything in the body is believed.
func (s *server) receiveVendorWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		writeError(w, s.log, http.StatusBadRequest, codeValidation, "could not read the request body")
		return
	}

	timestamp := r.Header.Get(headerTimestamp)
	if !s.secrets.verify(timestamp, body, r.Header.Get(headerSignature)) {
		writeError(w, s.log, http.StatusUnauthorized, codeUnauthenticated,
			"the signature does not match the webhook secret")
		return
	}

	if err := checkWebhookFreshness(timestamp); err != nil {
		writeError(w, s.log, http.StatusBadRequest, codeValidation, err.Error())
		return
	}

	var msg webhookMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		writeError(w, s.log, http.StatusBadRequest, codeValidation, "the body is not a JSON object")
		return
	}
	if details := validateWebhookMessage(msg); len(details) > 0 {
		writeValidationError(w, s.log, details)
		return
	}

	applied, change, err := s.store.ApplyWebhookStatus(r.Context(), msg.ID, msg.TaskID, msg.Status)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.notFound(w)
		return
	case err != nil:
		s.fail(w, r, "apply webhook status", err)
		return
	}

	if !applied {
		writeJSON(w, s.log, http.StatusOK, map[string]string{"status": "duplicate"})
		return
	}

	writeJSON(w, s.log, http.StatusAccepted, map[string]string{"status": "accepted"})
	s.notifier.Notify(change)
}

// verify reports whether signature is a valid HMAC of "<timestamp>.<body>" under any secret this
// server currently accepts.
func (s webhookSecrets) verify(timestamp string, body []byte, signature string) bool {
	want, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil || len(want) == 0 {
		return false
	}
	for _, secret := range []string{s.current, s.previous} {
		if secret == "" {
			continue
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(timestamp + "."))
		mac.Write(body)
		if hmac.Equal(mac.Sum(nil), want) {
			return true
		}
	}
	return false
}

func checkWebhookFreshness(timestamp string) error {
	seconds, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return errors.New(headerTimestamp + " must be a Unix timestamp in seconds")
	}
	drift := math.Abs(time.Since(time.Unix(seconds, 0)).Seconds())
	if drift > webhookSkew.Seconds() {
		return errors.New(headerTimestamp + " is too far from this server's clock")
	}
	return nil
}

func validateWebhookMessage(msg webhookMessage) []fieldError {
	var details []fieldError
	if strings.TrimSpace(msg.ID) == "" {
		details = append(details, fieldError{Field: "id", Message: "is required and identifies this message"})
	}
	if msg.Type != "task.status" {
		details = append(details, fieldError{Field: "type", Message: `must be "task.status"`})
	}
	if _, err := uuid.Parse(msg.TaskID); err != nil {
		details = append(details, fieldError{Field: "taskId", Message: "must name an existing task"})
	}
	if !store.ValidStatus(msg.Status) {
		details = append(details, fieldError{
			Field:   "status",
			Message: "must be one of " + strings.Join(store.Statuses, ", "),
		})
	}
	return details
}
