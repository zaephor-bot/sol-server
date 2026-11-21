package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zaephor-bot/sol-server/internal/moon"
)

// The fractions below are the US Naval Observatory's published values
// for 0h UT, from the "Fraction of the Moon Illuminated" table,
// https://aa.usno.navy.mil/data/MoonFraction (two decimals); see
// internal/moon/moon_test.go for the full anchor table and its
// corroboration by known eclipse instants.
func TestMoonEndpointAnswersWithUSNOValues(t *testing.T) {
	anchors := []struct {
		date  string
		phase string
		usno  float64
	}{
		{date: "2000-01-21", phase: "Full Moon", usno: 1.00},
		{date: "2001-06-21", phase: "New Moon", usno: 0.00},
		{date: "2011-07-05", phase: "Waxing Crescent", usno: 0.16},
		{date: "2011-12-18", phase: "Last Quarter", usno: 0.51},
	}

	mux := Routes()
	for _, a := range anchors {
		t.Run(a.date, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/moon?date="+a.date, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("GET /v1/moon?date=%s = %d, want %d (body: %s)",
					a.date, rec.Code, http.StatusOK, rec.Body.String())
			}

			// The contract keys are frozen: exactly date,
			// illumination, phase — nothing more, nothing less.
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
				t.Fatalf("response is not a JSON object: %v (body: %s)", err, rec.Body.String())
			}
			if len(fields) != 3 {
				t.Fatalf("response has %d keys (%v), want exactly date, illumination, phase",
					len(fields), fieldNames(fields))
			}
			for _, key := range []string{"date", "illumination", "phase"} {
				if _, ok := fields[key]; !ok {
					t.Fatalf("response is missing key %q (has %v)", key, fieldNames(fields))
				}
			}

			var body moon.Info
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response does not decode into the library's answer: %v (body: %s)", err, rec.Body.String())
			}
			if body.Date != a.date {
				t.Fatalf("date = %q, want it to echo the request date %q", body.Date, a.date)
			}
			if body.Phase != a.phase {
				t.Fatalf("phase = %q, want %q (USNO's phase for the date)", body.Phase, a.phase)
			}
			if diff := math.Abs(body.Illumination - a.usno); diff > 0.01+1e-9 {
				t.Fatalf("illumination = %.4f, USNO publishes %.2f for %s (difference %.4f exceeds 0.01)",
					body.Illumination, a.usno, a.date, diff)
			}

			// The endpoint must wrap the library, not compute its own
			// answer: the body has to equal what the library returns
			// for the parsed date.
			day, err := time.Parse(time.DateOnly, a.date)
			if err != nil {
				t.Fatalf("anchor %q does not parse: %v", a.date, err)
			}
			if want := moon.For(day); body != want {
				t.Fatalf("response = %+v, want the library's answer %+v", body, want)
			}
		})
	}
}

func fieldNames(fields map[string]json.RawMessage) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	return names
}

func TestMoonEndpointAcceptsLeapDay(t *testing.T) {
	rec := httptest.NewRecorder()
	Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/moon?date=2024-02-29", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/moon?date=2024-02-29 = %d, want %d (2024 is a leap year; body: %s)",
			rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestMoonEndpointRejectsMalformedDates(t *testing.T) {
	bad := []string{
		"",                     // missing altogether / empty value
		"not-a-date",           // not ISO
		"2024-13-01",           // impossible month
		"2024-00-10",           // impossible month
		"2024-02-30",           // impossible day
		"2024-04-31",           // impossible day
		"2023-02-29",           // February 29th of a common year
		"2024-1-1",             // not zero-padded
		"01-01-2024",           // wrong field order
		"20240102",             // no separators
		"2024-01-02T03:04:05Z", // a timestamp, not a calendar date
		"%202024-01-02",        // stray whitespace
		"2024-01-02%20",        // stray whitespace
		"999999-01-01",         // not a four-digit year
	}
	for _, date := range bad {
		t.Run(fmt.Sprintf("date=%q", date), func(t *testing.T) {
			rec := httptest.NewRecorder()
			Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/moon?date="+date, nil))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("GET /v1/moon?date=%q = %d, want %d (body: %s)",
					date, rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("400 body is not JSON with an error field: %v (body: %s)", err, rec.Body.String())
			}
			if body.Error == "" {
				t.Fatalf("400 body carries no human-readable message (body: %s)", rec.Body.String())
			}
		})
	}
}

func TestMoonEndpointRequiresDateParameter(t *testing.T) {
	for _, target := range []string{"/v1/moon", "/v1/moon?phase=full"} {
		rec := httptest.NewRecorder()
		Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET %s = %d, want %d: a missing date is rejected rather than defaulted to today (body: %s)",
				target, rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	}
}

func TestMoonEndpointRejectsNonGET(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		Routes().ServeHTTP(rec, httptest.NewRequest(method, "/v1/moon?date=2024-04-08", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s /v1/moon = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestIndexAdvertisesMoonEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	var body struct {
		Service   string   `json:"service"`
		Endpoints []string `json:"endpoints"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("index response does not decode: %v (body: %s)", err, rec.Body.String())
	}
	for _, endpoint := range body.Endpoints {
		if endpoint == "/v1/moon" {
			return
		}
	}
	t.Fatalf("index endpoints %v do not advertise /v1/moon", body.Endpoints)
}
