package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/zaephor-bot/sol-server/internal/moon"
)

// handleMoon serves GET /v1/moon?date=YYYY-MM-DD.
//
// The response body is the library's answer, written back as-is:
//
//	{"date": "2024-04-08", "illumination": 0.007, "phase": "New Moon"}
//
// Contract (the keys are frozen and will not be renamed):
//
//   - date is a required query parameter holding an ISO calendar
//     date (YYYY-MM-DD). A request without it — like one carrying a
//     malformed date: a non-ISO string, or an impossible month or
//     day — is answered with 400 and a human-readable error. The
//     endpoint never substitutes "today" for a missing date, so
//     answers stay deterministic.
//   - illumination is the raw fraction of the lunar disk lit at
//     00:00 UTC of the requested date, a number in [0, 1]; USNO
//     publishes the same quantity rounded to two decimals.
//   - phase is one of the eight classical phase names.
//
// The endpoint holds no astronomy of its own: it parses the date,
// asks the moon library for the values, and writes back what the
// library returned.
func handleMoon(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{
			Error: `missing required query parameter "date": pass a UTC calendar date in YYYY-MM-DD form, e.g. ?date=2024-04-08`,
		})
		return
	}
	day, err := parseCalendarDate(date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, moon.For(day))
}

// errorBody is the shape of every error response the service sends.
type errorBody struct {
	Error string `json:"error"`
}

// parseCalendarDate parses a strict YYYY-MM-DD calendar date and
// returns its 00:00 UTC instant.
//
// time.Parse accepts only the zero-padded ISO shape and rejects
// impossible calendar dates (month 13, April 31st, February 29th in
// a common year). The round-trip check afterwards additionally
// rejects anything Parse might have silently normalised, so the
// validation never depends on matching stdlib error-message wording.
func parseCalendarDate(s string) (time.Time, error) {
	const shape = "an ISO calendar date in YYYY-MM-DD form, e.g. 2024-02-29"
	parsed, err := time.Parse(time.DateOnly, s)
	if err != nil || parsed.Format(time.DateOnly) != s {
		return time.Time{}, fmt.Errorf("invalid date %q: want %s", s, shape)
	}
	return parsed, nil
}
