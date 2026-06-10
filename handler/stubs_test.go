package handler_test

import (
	"context"
	"time"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/db"
)

// stubDB is a configurable in-test implementation of db.Client.
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
