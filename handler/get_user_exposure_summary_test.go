package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/exposure"
	"github.com/ctrl-hub/challenge/handler"
)

func TestGetUserExposureSummary(t *testing.T) {
	cases := []struct {
		name       string
		userID     string
		query      string
		setupDB    func(*stubDB)
		wantStatus int
	}{
		{
			name:   "valid user with no time filters returns 200",
			userID: testUserID,
			setupDB: func(s *stubDB) {
				s.users[testUserID] = &data.User{ID: testUserID, Name: "Alice"}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "valid user with RFC3339 time filters returns 200",
			userID: testUserID,
			query:  "?starting_at=2025-01-01T00:00:00Z&ending_at=2025-01-31T23:59:59Z",
			setupDB: func(s *stubDB) {
				s.users[testUserID] = &data.User{ID: testUserID, Name: "Alice"}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-UUID user ID returns 400",
			userID:     "not-a-valid-uuid",
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid starting_at format returns 400",
			userID:     testUserID,
			query:      "?starting_at=2025-01-01",
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid ending_at format returns 400",
			userID:     testUserID,
			query:      "?ending_at=not-a-date",
			setupDB:    func(_ *stubDB) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "user not found returns 404",
			userID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			setupDB: func(_ *stubDB) {
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

			url := "/users/" + tc.userID + "/exposure-summary" + tc.query
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()

			h.Routes().ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)",
					tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}
