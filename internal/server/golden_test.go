package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// The one pinned request of the /v1/events contract and the golden
// file holding its exact answer. The path spells the request so the
// file's provenance survives a glance at testdata/.
const (
	goldenEventsQuery = "/v1/events?lat=33.6&lon=-117.8&date=2026-06-21"
	goldenEventsFile  = "../../testdata/events-2026-06-21-lat33.6-lon-117.8.json"
)

// TestEventsGoldenFile pins the exact 200 body for the pinned request,
// byte for byte. The response is a contract other code builds on, so
// any change — a renamed key, a drifted value, a reformatted time, a
// dropped trailing newline — must fail here loudly. To regenerate
// after a deliberate contract change, run the handler once and write
// its body to the golden file; there is no -update flag on purpose:
// every rewrite of this file should be a reviewed, explainable diff.
func TestEventsGoldenFile(t *testing.T) {
	recorder := httptest.NewRecorder()
	Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, goldenEventsQuery, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want %d (body: %s)",
			goldenEventsQuery, recorder.Code, http.StatusOK, recorder.Body.String())
	}

	want, err := os.ReadFile(goldenEventsFile)
	if err != nil {
		t.Fatalf("reading the golden file: %v", err)
	}
	if got := recorder.Body.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("GET %s no longer matches %s.\n--- want (golden) ---\n%s--- got (handler) ---\n%s---",
			goldenEventsQuery, goldenEventsFile, want, got)
	}
}
