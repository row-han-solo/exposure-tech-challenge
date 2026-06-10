package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/exposure"
	"github.com/ctrl-hub/challenge/handler"
)

const (
	testUserID  = "713be58e-0d79-4df2-a85c-9f44ca513a7d"
	testEquipID = "2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"
)

func TestRecordExposure(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		setupDB    func(*stubDB)
		wantStatus int
	}{
		{
			name: "valid request creates exposure and returns 201",
			body: `{"equipment_id":"` + testEquipID + `","user_id":"` + testUserID + `","duration":30}`,
			setupDB: func(s *stubDB) {
				s.users[testUserID] = &data.User{ID: testUserID, Name: "Alice"}
				s.equipment[testEquipID] = &data.EquipmentItem{
					ID:                 testEquipID,
					Name:               "AirCat - Drill - 4337",
					VibrationMagnitude: 2.1,
				}
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "empty body returns 400",
			body:       ``,
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed JSON returns 400",
			body:       `{not valid json`,
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid equipment_id UUID returns 400",
			body:       `{"equipment_id":"not-a-uuid","user_id":"` + testUserID + `","duration":30}`,
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid user_id UUID returns 400",
			body:       `{"equipment_id":"` + testEquipID + `","user_id":"not-a-uuid","duration":30}`,
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero duration returns 400",
			body:       `{"equipment_id":"` + testEquipID + `","user_id":"` + testUserID + `","duration":0}`,
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative duration returns 400",
			body:       `{"equipment_id":"` + testEquipID + `","user_id":"` + testUserID + `","duration":-5}`,
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "equipment not found returns 404",
			body: `{"equipment_id":"` + testEquipID + `","user_id":"` + testUserID + `","duration":30}`,
			setupDB: func(s *stubDB) {
				s.users[testUserID] = &data.User{ID: testUserID, Name: "Alice"}
				// equipment not seeded
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "user not found returns 404",
			body: `{"equipment_id":"` + testEquipID + `","user_id":"` + testUserID + `","duration":30}`,
			setupDB: func(s *stubDB) {
				s.equipment[testEquipID] = &data.EquipmentItem{
					ID:                 testEquipID,
					Name:               "AirCat - Drill - 4337",
					VibrationMagnitude: 2.1,
				}
				// user not seeded
			},
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

			req := httptest.NewRequest(http.MethodPost, "/exposure", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.Routes().ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)",
					tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}
