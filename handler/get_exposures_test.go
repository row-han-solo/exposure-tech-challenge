package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/exposure"
	"github.com/ctrl-hub/challenge/handler"
)

func TestGetExposures(t *testing.T) {
	cases := []struct {
		name         string
		setupDB      func(*stubDB)
		wantStatus   int
		wantMinCount int
	}{
		{
			name: "returns 200 with all stored exposures",
			setupDB: func(s *stubDB) {
				s.exposures["e1"] = &data.Exposure{ID: "e1", Duration: 30, CreatedAt: time.Now()}
				s.exposures["e2"] = &data.Exposure{ID: "e2", Duration: 60, CreatedAt: time.Now()}
			},
			wantStatus:   http.StatusOK,
			wantMinCount: 2,
		},
		{
			name:         "returns 200 with empty array when no exposures exist",
			setupDB:      func(_ *stubDB) {},
			wantStatus:   http.StatusOK,
			wantMinCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sdb := newStubDB()
			if tc.setupDB != nil {
				tc.setupDB(sdb)
			}
			h := handler.New(sdb, exposure.NoopPublisher{})

			req := httptest.NewRequest(http.MethodGet, "/exposure", nil)
			rec := httptest.NewRecorder()

			h.Routes().ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}

			var results []data.Exposure
			if err := json.NewDecoder(rec.Body).Decode(&results); err != nil {
				t.Fatalf("could not decode response: %v", err)
			}
			if len(results) < tc.wantMinCount {
				t.Errorf("expected at least %d exposures, got %d", tc.wantMinCount, len(results))
			}
		})
	}
}
