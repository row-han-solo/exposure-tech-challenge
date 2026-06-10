package exposure_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/db"
	"github.com/ctrl-hub/challenge/exposure"
)

// stubDB is an in-test implementation of db.Client backed by plain maps.
type stubDB struct {
	users     map[string]*data.User
	equipment map[string]*data.EquipmentItem
	exposures map[string]*data.Exposure
}

func newStubDB() *stubDB {
	return &stubDB{
		users:     make(map[string]*data.User),
		equipment: make(map[string]*data.EquipmentItem),
		exposures: make(map[string]*data.Exposure),
	}
}

func (s *stubDB) GetUser(_ context.Context, id string) (*data.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	return u, nil
}

func (s *stubDB) GetEquipment(_ context.Context, id string) (*data.EquipmentItem, error) {
	eq, ok := s.equipment[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	return eq, nil
}

func (s *stubDB) ListEquipment(_ context.Context) ([]*data.EquipmentItem, error) {
	items := make([]*data.EquipmentItem, 0, len(s.equipment))
	for _, eq := range s.equipment {
		items = append(items, eq)
	}
	return items, nil
}

func (s *stubDB) CreateExposure(_ context.Context, e *data.Exposure) error {
	s.exposures[e.ID] = e
	return nil
}

func (s *stubDB) GetExposure(_ context.Context, id string) (*data.Exposure, error) {
	e, ok := s.exposures[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	return e, nil
}

func (s *stubDB) ListExposures(_ context.Context) ([]*data.Exposure, error) {
	items := make([]*data.Exposure, 0, len(s.exposures))
	for _, e := range s.exposures {
		items = append(items, e)
	}
	return items, nil
}

func (s *stubDB) GetExposuresByUser(_ context.Context, userID string, start, end *time.Time) ([]*data.Exposure, error) {
	var items []*data.Exposure
	for _, e := range s.exposures {
		if e.User.ID != userID {
			continue
		}
		if start != nil && e.CreatedAt.Before(*start) {
			continue
		}
		if end != nil && e.CreatedAt.After(*end) {
			continue
		}
		items = append(items, e)
	}
	return items, nil
}

// stubPublisher captures published event names for assertion.
type stubPublisher struct {
	events []string
}

func (s *stubPublisher) Publish(_ context.Context, event string, _ any) error {
	s.events = append(s.events, event)
	return nil
}

func (s *stubPublisher) hasEvent(name string) bool {
	for _, e := range s.events {
		if e == name {
			return true
		}
	}
	return false
}

const (
	validUserID  = "713be58e-0d79-4df2-a85c-9f44ca513a7d"
	validEquipID = "2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"
)

func seedValidUser(s *stubDB) {
	s.users[validUserID] = &data.User{ID: validUserID, Name: "Alice Smith"}
}

func seedAirCatDrill(s *stubDB) {
	s.equipment[validEquipID] = &data.EquipmentItem{
		ID:                 validEquipID,
		Name:               "AirCat - Drill - 4337",
		VibrationMagnitude: 2.1,
	}
}

// ---- Record tests ----

func TestRecord(t *testing.T) {
	cases := []struct {
		name        string
		req         data.RecordExposureRequest
		setupDB     func(*stubDB)
		wantErr     error
		checkResult func(*testing.T, *data.Exposure)
	}{
		{
			name: "valid request calculates a8 and points and stores exposure",
			req: data.RecordExposureRequest{
				EquipmentID: validEquipID,
				UserID:      validUserID,
				Duration:    60,
			},
			setupDB: func(s *stubDB) {
				seedValidUser(s)
				seedAirCatDrill(s)
			},
			checkResult: func(t *testing.T, e *data.Exposure) {
				t.Helper()
				if e.ID == "" {
					t.Error("expected non-empty ID")
				}
				if e.A8 == 0 {
					t.Error("expected non-zero A8")
				}
				if e.Points == 0 {
					t.Error("expected non-zero Points")
				}
				if e.Duration != 60 {
					t.Errorf("expected duration 60, got %d", e.Duration)
				}
				if e.User.ID != validUserID {
					t.Errorf("expected user ID %q, got %q", validUserID, e.User.ID)
				}
				if e.Equipment.ID != validEquipID {
					t.Errorf("expected equipment ID %q, got %q", validEquipID, e.Equipment.ID)
				}
			},
		},
		{
			name: "equipment not found returns ErrNotFound",
			req: data.RecordExposureRequest{
				EquipmentID: validEquipID,
				UserID:      validUserID,
				Duration:    30,
			},
			setupDB: func(s *stubDB) {
				seedValidUser(s)
				// equipment not seeded
			},
			wantErr: db.ErrNotFound,
		},
		{
			name: "user not found returns ErrNotFound",
			req: data.RecordExposureRequest{
				EquipmentID: validEquipID,
				UserID:      validUserID,
				Duration:    30,
			},
			setupDB: func(s *stubDB) {
				seedAirCatDrill(s)
				// user not seeded
			},
			wantErr: db.ErrNotFound,
		},
		{
			name: "exposure is persisted and retrievable after record",
			req: data.RecordExposureRequest{
				EquipmentID: validEquipID,
				UserID:      validUserID,
				Duration:    45,
			},
			setupDB: func(s *stubDB) {
				seedValidUser(s)
				seedAirCatDrill(s)
			},
			checkResult: func(t *testing.T, e *data.Exposure) {
				t.Helper()
				if e.CreatedAt.IsZero() {
					t.Error("expected non-zero CreatedAt timestamp")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sdb := newStubDB()
			if tc.setupDB != nil {
				tc.setupDB(sdb)
			}
			pub := &stubPublisher{}

			result, err := exposure.Record(context.Background(), sdb, pub, tc.req)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkResult != nil {
				tc.checkResult(t, result)
			}
		})
	}
}

func TestRecord_PublishesExposureRecordedEvent(t *testing.T) {
	sdb := newStubDB()
	seedValidUser(sdb)
	seedAirCatDrill(sdb)
	pub := &stubPublisher{}

	_, err := exposure.Record(context.Background(), sdb, pub, data.RecordExposureRequest{
		EquipmentID: validEquipID,
		UserID:      validUserID,
		Duration:    30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !pub.hasEvent("exposure.recorded") {
		t.Errorf("expected exposure.recorded event to be published, got: %v", pub.events)
	}
}

func TestRecord_PublishesEAVEvent_WhenDailyThresholdReached(t *testing.T) {
	sdb := newStubDB()
	seedValidUser(sdb)
	// JCB Hydraulic Breaker: 4.0 m/s²
	jcbID := "jcb-001"
	sdb.equipment[jcbID] = &data.EquipmentItem{
		ID:                 jcbID,
		Name:               "JCB - Hydraulic Breaker - CEJCBHM25",
		VibrationMagnitude: 4.0,
	}

	now := time.Now().UTC()

	// Seed an existing exposure that puts the user at 90 points for today.
	// 60 min at 4.0 m/s² → points = (4/2.5)^2 * (1/8) * 100 = 32 points per hour
	// We seed 90 points manually to set up the threshold scenario.
	sdb.exposures["prior"] = &data.Exposure{
		ID:        "prior",
		User:      data.User{ID: validUserID},
		A8:        1.0,
		Points:    90,
		CreatedAt: now,
	}

	pub := &stubPublisher{}

	// Adding 60 min on JCB (32 points) pushes total to 122 → crosses EAV (100).
	_, err := exposure.Record(context.Background(), sdb, pub, data.RecordExposureRequest{
		EquipmentID: jcbID,
		UserID:      validUserID,
		Duration:    60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !pub.hasEvent("exposure.eav_reached") {
		t.Errorf("expected exposure.eav_reached event, got: %v", pub.events)
	}
}

// ---- GetSummary tests ----

func TestGetSummary(t *testing.T) {
	now := time.Now().UTC()

	cases := []struct {
		name        string
		setupDB     func(*stubDB)
		start       *time.Time
		end         *time.Time
		wantErr     error
		checkResult func(*testing.T, *data.ExposureSummary)
	}{
		{
			name: "aggregates points additively and a8 as root-sum-of-squares",
			setupDB: func(s *stubDB) {
				seedValidUser(s)
				s.exposures["e1"] = &data.Exposure{
					ID: "e1", User: data.User{ID: validUserID},
					A8: 1.0, Points: 25, CreatedAt: now,
				}
				s.exposures["e2"] = &data.Exposure{
					ID: "e2", User: data.User{ID: validUserID},
					A8: 2.0, Points: 75, CreatedAt: now,
				}
			},
			checkResult: func(t *testing.T, s *data.ExposureSummary) {
				t.Helper()
				if s.Points != 100 {
					t.Errorf("expected 100 points, got %v", s.Points)
				}
				// a8 = sqrt(1² + 2²) = sqrt(5)
				want := math.Sqrt(5)
				if math.Abs(s.A8-want) > 0.0001 {
					t.Errorf("expected A8 %v, got %v", want, s.A8)
				}
				if s.User.ID != validUserID {
					t.Errorf("expected user ID %q, got %q", validUserID, s.User.ID)
				}
			},
		},
		{
			name: "returns zero summary for user with no exposures in time window",
			setupDB: func(s *stubDB) {
				seedValidUser(s)
				yesterday := now.Add(-25 * time.Hour)
				s.exposures["old"] = &data.Exposure{
					ID: "old", User: data.User{ID: validUserID},
					A8: 1.5, Points: 50, CreatedAt: yesterday,
				}
			},
			start: func() *time.Time { t := now.Add(-1 * time.Hour); return &t }(),
			end:   func() *time.Time { t := now.Add(1 * time.Hour); return &t }(),
			checkResult: func(t *testing.T, s *data.ExposureSummary) {
				t.Helper()
				if s.Points != 0 {
					t.Errorf("expected 0 points, got %v", s.Points)
				}
				if s.A8 != 0 {
					t.Errorf("expected 0 A8, got %v", s.A8)
				}
			},
		},
		{
			name: "user not found returns ErrNotFound",
			setupDB: func(s *stubDB) {
				// user not seeded
			},
			wantErr: db.ErrNotFound,
		},
		{
			name: "no time filter returns all user exposures",
			setupDB: func(s *stubDB) {
				seedValidUser(s)
				for _, id := range []string{"e1", "e2", "e3"} {
					s.exposures[id] = &data.Exposure{
						ID: id, User: data.User{ID: validUserID},
						A8: 1.0, Points: 10, CreatedAt: now.Add(-time.Duration(len(id)) * time.Hour),
					}
				}
			},
			checkResult: func(t *testing.T, s *data.ExposureSummary) {
				t.Helper()
				if s.Points != 30 {
					t.Errorf("expected 30 points, got %v", s.Points)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sdb := newStubDB()
			if tc.setupDB != nil {
				tc.setupDB(sdb)
			}

			result, err := exposure.GetSummary(context.Background(), sdb, validUserID, tc.start, tc.end)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkResult != nil {
				tc.checkResult(t, result)
			}
		})
	}
}

// ---- Get tests ----

func TestGet(t *testing.T) {
	cases := []struct {
		name    string
		setupDB func(*stubDB)
		id      string
		wantErr error
	}{
		{
			name: "returns exposure when found",
			setupDB: func(s *stubDB) {
				s.exposures["exp-1"] = &data.Exposure{ID: "exp-1"}
			},
			id: "exp-1",
		},
		{
			name:    "returns ErrNotFound when exposure does not exist",
			setupDB: func(s *stubDB) {},
			id:      "missing",
			wantErr: db.ErrNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sdb := newStubDB()
			tc.setupDB(sdb)

			got, err := exposure.Get(context.Background(), sdb, tc.id)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tc.id {
				t.Errorf("expected ID %q, got %q", tc.id, got.ID)
			}
		})
	}
}

// ---- List tests ----

func TestList(t *testing.T) {
	cases := []struct {
		name      string
		setupDB   func(*stubDB)
		wantCount int
	}{
		{
			name: "returns all exposures",
			setupDB: func(s *stubDB) {
				s.exposures["e1"] = &data.Exposure{ID: "e1"}
				s.exposures["e2"] = &data.Exposure{ID: "e2"}
			},
			wantCount: 2,
		},
		{
			name:      "returns empty slice when no exposures exist",
			setupDB:   func(s *stubDB) {},
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sdb := newStubDB()
			tc.setupDB(sdb)

			got, err := exposure.List(context.Background(), sdb)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantCount {
				t.Errorf("expected %d exposures, got %d", tc.wantCount, len(got))
			}
		})
	}
}
