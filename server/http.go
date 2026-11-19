package gymserver

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	corei18n "code.pecheny.me/pecheny/dopesuite/dopecore/i18nstrings"
)

const maxJSONBody = 1 << 20

// statusError is a refusal with its own status and words.
type statusError struct {
	code int
	msg  string
}

func (e statusError) Error() string { return strconv.Itoa(e.code) + " " + e.msg }

// errorOf is the one place a failure becomes a status and the words a person
// reads: a statusError as itself, a UserError as a 400 carrying its own, and
// anything else as one generic line over a log entry (root docs/adr/0006).
func errorOf(err error) (int, string) {
	var st statusError
	if errors.As(err, &st) {
		return st.code, st.msg
	}
	msg, forUser := corei18n.Reveal(err, str.Server.Error.Internal())
	if forUser {
		return http.StatusBadRequest, msg
	}
	log.Printf("gym: %v", err)
	return http.StatusInternalServerError, msg
}

// api adapts a JSON handler: its error is written as a plain-text status, which
// is what the kit's login script reads.
func api(h func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			code, msg := errorOf(err)
			http.Error(w, msg, code)
		}
	}
}

// writeJSON writes v uncached: a session's answers are per person.
func writeJSON(w http.ResponseWriter, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
	return nil
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return corei18n.User(str.Server.Error.BadRequest())
	}
	return nil
}

func logDropped(what string, err error) {
	if err != nil {
		log.Printf("gym: %s: %v", what, err)
	}
}
