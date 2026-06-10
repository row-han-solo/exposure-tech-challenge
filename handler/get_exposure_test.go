package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/exposure"
	"github.com/ctrl-hub/challenge/handler"
)

func TestGetExposure(t *testing.T) {
	storedExposureID := "e8f7b50c-cc18-42f9-a275-0b4ead73f806"

	cases := []struct {
		name       string
		exposureID string
		setupDB    func(*stubDB)
		wantStatus int
	}{
		{
			name:       "valid exposure ID returns 200",
			exposureID: storedExposureID,
			setupDB: func(s *stubDB) {
				s.exposures[storedExposureID] = &data.Exposure{
					ID:        storedExposureID,
					Duration:  30,
					CreatedAt: time.Now(),
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-UUID exposure ID returns 400",
			exposureID: "not-a-valid-uuid",
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "exposure not found returns 404",
			exposureID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sdb := newStubDB()
			if tc.setupDB != nil {
				tc.setupDB(sdb)
			}
			h := handler.New(sdb, exposure.NoopPublisher{})

			req := httptest.NewRequest(http.MethodGet, "/exposure/"+tc.exposureID, nil)
			rec := httptest.NewRecorder()

			h.Routes().ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)",
					tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}
