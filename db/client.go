package db

import (
	"context"
	"errors"
	"time"

	"github.com/ctrl-hub/challenge/data"
)

// ErrNotFound is returned by Client methods when the requested resource does not exist.
var ErrNotFound = errors.New("not found")

// Client abstracts all persistence operations for the HAVS service.
type Client interface {
	GetUser(ctx context.Context, id string) (*data.User, error)

	GetEquipment(ctx context.Context, id string) (*data.EquipmentItem, error)
	ListEquipment(ctx context.Context) ([]*data.EquipmentItem, error)

	CreateExposure(ctx context.Context, e *data.Exposure) error
	GetExposure(ctx context.Context, id string) (*data.Exposure, error)
	ListExposures(ctx context.Context) ([]*data.Exposure, error)
	GetExposuresByUser(ctx context.Context, userID string, start, end *time.Time) ([]*data.Exposure, error)
}
