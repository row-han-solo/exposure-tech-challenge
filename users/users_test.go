package users_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/db"
	"github.com/ctrl-hub/challenge/users"
)

// stubClient is a minimal in-test implementation of db.Client.
type stubClient struct {
	user *data.User
	err  error
}

func (s *stubClient) GetUser(_ context.Context, _ string) (*data.User, error) {
	return s.user, s.err
}

func (s *stubClient) GetEquipment(_ context.Context, _ string) (*data.EquipmentItem, error) {
	return nil, nil
}

func (s *stubClient) ListEquipment(_ context.Context) ([]*data.EquipmentItem, error) {
	return nil, nil
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
		userID  string
		wantErr error
		wantID  string
	}{
		{
			name:   "returns user when found",
			client: &stubClient{user: &data.User{ID: "abc", Name: "Alice"}},
			userID: "abc",
			wantID: "abc",
		},
		{
			name:    "returns ErrNotFound when user does not exist",
			client:  &stubClient{err: db.ErrNotFound},
			userID:  "missing",
			wantErr: db.ErrNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := users.Get(context.Background(), tc.client, tc.userID)

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
				t.Errorf("expected user ID %q, got %q", tc.wantID, got.ID)
			}
		})
	}
}
