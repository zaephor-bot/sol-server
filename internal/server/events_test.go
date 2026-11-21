package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// TestEventsValidation pins the 400 contract: lat, lon and date are
// all required, decimal degrees are range-checked (±90, ±180), and
// the date must be a strict YYYY-MM-DD calendar day. Every error body
// carries a human-readable message in the fixed {"error": …} shape.
func TestEventsValidation(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		wantMentions   []string // fragments the error message must contain
		wantNotMention string   // fragment the error message must not contain
	}{
		{
			name:         "missing lat",
			query:        "/v1/events?lon=-117.8&date=2026-06-21",
			wantMentions: []string{"missing", `"lat"`},
		},
		{
			name:         "missing lon",
			query:        "/v1/events?lat=33.6&date=2026-06-21",
			wantMentions: []string{"missing", `"lon"`},
		},
		{
			name:         "missing date",
			query:        "/v1/events?lat=33.6&lon=-117.8",
			wantMentions: []string{"missing", `"date"`},
		},
		{
			name:           "empty lat",
			query:          "/v1/events?lat=&lon=-117.8&date=2026-06-21",
			wantMentions:   []string{"missing", `"lat"`},
			wantNotMention: "out of range",
		},
		{
			name:         "lat is not a number",
			query:        "/v1/events?lat=porridge&lon=-117.8&date=2026-06-21",
			wantMentions: []string{"invalid lat", "porridge"},
		},
		{
			name:         "lat is NaN",
			query:        "/v1/events?lat=NaN&lon=-117.8&date=2026-06-21",
			wantMentions: []string{"invalid lat"},
		},
		{
			name:         "lat is infinite",
			query:        "/v1/events?lat=Inf&lon=-117.8&date=2026-06-21",
			wantMentions: []string{"invalid lat"},
		},
		{
			name:         "lat above 90",
			query:        "/v1/events?lat=90.5&lon=-117.8&date=2026-06-21",
			wantMentions: []string{"out of range", "-90", "90"},
		},
		{
			name:         "lat below -90",
			query:        "/v1/events?lat=-90.0001&lon=-117.8&date=2026-06-21",
			wantMentions: []string{"out of range"},
		},
		{
			name:         "lon above 180",
			query:        "/v1/events?lat=33.6&lon=181&date=2026-06-21",
			wantMentions: []string{"out of range", "-180", "180"},
		},
		{
			name:         "lon below -180",
			query:        "/v1/events?lat=33.6&lon=-415.2&date=2026-06-21",
			wantMentions: []string{"out of range"},
		},
		{
			name:         "lon is not a number",
			query:        "/v1/events?lat=33.6&lon=here&date=2026-06-21",
			wantMentions: []string{"invalid lon", "here"},
		},
		{
			name:         "date is not ISO shaped",
			query:        "/v1/events?lat=33.6&lon=-117.8&date=06/21/2026",
			wantMentions: []string{"invalid date", "YYYY-MM-DD"},
		},
		{
			name:         "date is not zero-padded",
			query:        "/v1/events?lat=33.6&lon=-117.8&date=2026-6-21",
			wantMentions: []string{"invalid date"},
		},
		{
			name:         "date is an impossible calendar day",
			query:        "/v1/events?lat=33.6&lon=-117.8&date=2026-02-30",
			wantMentions: []string{"invalid date"},
		},
		{
			name:         "date is not a date at all",
			query:        "/v1/events?lat=33.6&lon=-117.8&date=tomorrow",
			wantMentions: []string{"invalid date", "tomorrow"},
		},
	}

	mux := Routes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.query, nil))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("GET %s = %d, want %d (body: %s)",
					tt.query, rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body is not {\"error\": …}: %v (body: %s)", err, rec.Body.String())
			}
			for _, fragment := range tt.wantMentions {
				if !strings.Contains(body.Error, fragment) {
					t.Fatalf("error message %q does not mention %q", body.Error, fragment)
				}
			}
			if tt.wantNotMention != "" && strings.Contains(body.Error, tt.wantNotMention) {
				t.Fatalf("error message %q unexpectedly mentions %q", body.Error, tt.wantNotMention)
			}
		})
	}
}

// TestEventsNoSunEvents pins the 422 contract: a well-formed request
// for a day and place where nothing happens at all — no sunrise, no
// sunset, and not one window — is answered with 422 and a message
// saying there are no sun events that day. The polar summer and the
// polar winter both take this path, at either pole.
func TestEventsNoSunEvents(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "polar summer, north pole", query: "/v1/events?lat=89&lon=0&date=2026-06-21"},
		{name: "polar winter, north pole", query: "/v1/events?lat=89&lon=0&date=2026-12-21"},
		{name: "polar winter, south pole", query: "/v1/events?lat=-89&lon=0&date=2026-06-21"},
		{name: "polar summer, south pole", query: "/v1/events?lat=-89&lon=0&date=2026-12-21"},
	}

	mux := Routes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.query, nil))

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("GET %s = %d, want %d (body: %s)",
					tt.query, rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body is not {\"error\": …}: %v (body: %s)", err, rec.Body.String())
			}
			for _, fragment := range []string{"no sun events", "2026-", "polar"} {
				if !strings.Contains(body.Error, fragment) {
					t.Fatalf("error message %q does not mention %q", body.Error, fragment)
				}
			}
		})
	}
}

// TestEventsShape pins the 200 contract's key set, top to bottom: the
// response carries exactly the frozen keys, states its timezone and
// UTC offset, and every event a mid-latitude summer day holds is
// present and non-null.
func TestEventsShape(t *testing.T) {
	rec := httptest.NewRecorder()
	Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/v1/events?lat=33.6&lon=-117.8&date=2026-06-21", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/events = %d, want %d (body: %s)",
			rec.Code, http.StatusOK, rec.Body.String())
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a JSON object: %v (body: %s)", err, rec.Body.String())
	}
	wantTopLevel := []string{
		"latitude", "longitude", "date", "timezone", "utc_offset",
		"sunrise", "sunset", "solar_noon",
		"golden_hour", "blue_hour", "civil_twilight",
	}
	if got := fieldNames(body); !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(wantTopLevel))) {
		t.Fatalf("top-level keys = %v, want exactly %v", got, wantTopLevel)
	}

	for key, want := range map[string]string{
		"date":       `"2026-06-21"`,
		"timezone":   `"UTC"`,
		"utc_offset": `"+00:00"`,
		"sunrise":    `"2026-06-21T`,
		"sunset":     `"2026-06-2`,
		"solar_noon": `"2026-06-21T`,
	} {
		if got := string(body[key]); len(got) < len(want) || got[:len(want)] != want {
			t.Fatalf("%s = %s, want it to start with %s", key, got, want)
		}
	}

	for key, wantKeys := range map[string][]string{
		"golden_hour":    {"morning", "evening"},
		"blue_hour":      {"dawn", "dusk"},
		"civil_twilight": {"start", "end"},
	} {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(body[key], &nested); err != nil {
			t.Fatalf("%s is not a JSON object: %v (raw: %s)", key, err, body[key])
		}
		if got := fieldNames(nested); !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(wantKeys))) {
			t.Fatalf("%s keys = %v, want exactly %v", key, got, wantKeys)
		}
		for _, edge := range wantKeys {
			if string(nested[edge]) == "null" {
				t.Fatalf("%s.%s is null on a mid-latitude summer day, want a time (body: %s)",
					key, edge, rec.Body.String())
			}
		}
	}

	// Every time in the response is RFC 3339 UTC, matching the stated
	// timezone and offset.
	var times []string
	for _, key := range []string{"sunrise", "sunset", "solar_noon"} {
		var s string
		if err := json.Unmarshal(body[key], &s); err != nil {
			t.Fatalf("%s is not a string: %v", key, err)
		}
		times = append(times, s)
	}
	var pair struct {
		Morning, Evening eventWindow
	}
	for _, key := range []string{"golden_hour", "blue_hour"} {
		if err := json.Unmarshal(body[key], &pair); err != nil {
			t.Fatalf("%s does not decode into windows: %v (raw: %s)", key, err, body[key])
		}
		for _, w := range []eventWindow{pair.Morning, pair.Evening} {
			times = append(times, w.Start, w.End)
		}
	}
	for _, s := range times {
		if !strings.Contains(s, "T") || !strings.HasSuffix(s, "Z") {
			t.Fatalf("time %q is not an RFC 3339 UTC instant", s)
		}
	}
}

// TestEventsPartialDay pins the null contract: a day that holds some
// events but not all — here a polar-night day whose sun peaks inside
// the blue-hour band, so both blue hours happen while no horizon
// event does — answers 200 with null for every missing event.
func TestEventsPartialDay(t *testing.T) {
	rec := httptest.NewRecorder()
	Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/v1/events?lat=70&lon=0&date=2026-12-21", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/events = %d, want %d (body: %s)",
			rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Sunrise       *string           `json:"sunrise"`
		Sunset        *string           `json:"sunset"`
		SolarNoon     *string           `json:"solar_noon"`
		GoldenHour    goldenHourWindows `json:"golden_hour"`
		BlueHour      blueHourWindows   `json:"blue_hour"`
		CivilTwilight civilTwilightSpan `json:"civil_twilight"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response does not decode: %v (body: %s)", err, rec.Body.String())
	}

	if body.Sunrise != nil || body.Sunset != nil {
		t.Fatalf("sunrise/sunset = %v/%v, want null/null: the sun never rises at 70°N on the winter solstice",
			body.Sunrise, body.Sunset)
	}
	if body.SolarNoon == nil {
		t.Fatal("solar_noon is null, want a time: the transit happens even on a polar night")
	}
	if body.GoldenHour.Morning != nil || body.GoldenHour.Evening != nil {
		t.Fatalf("golden hour windows = %v/%v, want null/null: the sun never nears the horizon",
			body.GoldenHour.Morning, body.GoldenHour.Evening)
	}
	if body.BlueHour.Dawn == nil || body.BlueHour.Dusk == nil {
		t.Fatalf("blue hour windows = %v/%v, want both present: the sun peaks inside the -6…-4° band",
			body.BlueHour.Dawn, body.BlueHour.Dusk)
	}
	if body.CivilTwilight.Start != nil || body.CivilTwilight.End != nil {
		t.Fatalf("civil twilight = %v/%v, want null/null: the sun never bridges -6° and the horizon",
			body.CivilTwilight.Start, body.CivilTwilight.End)
	}
}
