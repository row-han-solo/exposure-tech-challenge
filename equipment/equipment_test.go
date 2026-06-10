package equipment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/db"
	"github.com/ctrl-hub/challenge/equipment"
)

// stubClient is a minimal in-test implementation of db.Client.
type stubClient struct {
	item  *data.EquipmentItem
	items []*data.EquipmentItem
	err   error
}

func (s *stubClient) GetUser(_ context.Context, _ string) (*data.User, error) {
	return nil, nil
}

func (s *stubClient) GetEquipment(_ context.Context, _ string) (*data.EquipmentItem, error) {
	return s.item, s.err
}

func (s *stubClient) ListEquipment(_ context.Context) ([]*data.EquipmentItem, error) {
	return s.items, s.err
}

func (s *stubClient) CreateExposure(_ context.Context, _ *data.Exposure) error {
	return nil
}

func (s *stubClient) GetExposure(_ context.Context, _ string) (*data.Exposure, error) {
	return nil, nil
}

func (s *stubClient) ListExposures(_ context.Context) ([]*data.Exposure, error) {
	return nil, nil
}

func (s *stubClient) GetExposuresByUser(_ context.Context, _ string, _, _ *time.Time) ([]*data.Exposure, error) {
	return nil, nil
}

func TestGet(t *testing.T) {
	cases := []struct {
		name    string
		client  db.Client
		wantErr error
		wantID  string
	}{
		{
			name: "returns equipment when found",
			client: &stubClient{item: &data.EquipmentItem{
				ID:                 "eq-1",
				Name:               "AirCat - Drill - 4337",
				VibrationMagnitude: 2.1,
			}},
			wantID: "eq-1",
		},
		{
			name:    "returns ErrNotFound when equipment does not exist",
			client:  &stubClient{err: db.ErrNotFound},
			wantErr: db.ErrNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := equipment.Get(context.Background(), tc.client, "eq-1")

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tc.wantID {
				t.Errorf("expected ID %q, got %q", tc.wantID, got.ID)
			}
		})
	}
}

func TestList(t *testing.T) {
	cases := []struct {
		name      string
		client    db.Client
		wantCount int
		wantErr   error
	}{
		{
			name: "returns all equipment items",
			client: &stubClient{items: []*data.EquipmentItem{
				{ID: "eq-1", Name: "AirCat - Drill - 4337", VibrationMagnitude: 2.1},
				{ID: "eq-2", Name: "JCB - Hydraulic Breaker - CEJCBHM25", VibrationMagnitude: 4.0},
			}},
			wantCount: 2,
		},
		{
			name:      "returns empty slice when no equipment exists",
			client:    &stubClient{items: []*data.EquipmentItem{}},
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := equipment.List(context.Background(), tc.client)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantCount {
				t.Errorf("expected %d items, got %d", tc.wantCount, len(got))
			}
		})
	}
}
