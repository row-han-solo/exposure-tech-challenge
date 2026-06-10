package exposure

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/db"
	"github.com/ctrl-hub/challenge/equipment"
	"github.com/ctrl-hub/challenge/users"
	"github.com/ctrl-hub/challenge/utils"
)

// EAV — Exposure Action Value (Manager Action Recommended)
// ELV — Exposure Limit Value (Compliance Limit Reached)

const (
	thresholdEAVA8     = 2.5 // m/s²
	thresholdELVA8     = 5.0 // m/s²
	thresholdEAVPoints = 100.0
	thresholdELVPoints = 400.0
)

// Publisher dispatches domain events to a message broker.
type Publisher interface {
	Publish(ctx context.Context, event string, payload any) error
}

// Record creates an exposure record for a user on a piece of equipment.
// It calculates the partial A(8) and points values, persists the record,
// and publishes threshold events when the user's daily EAV or ELV is reached.
func Record(ctx context.Context, client db.Client, pub Publisher, req data.RecordExposureRequest) (*data.Exposure, error) {
	eq, err := equipment.Get(ctx, client, req.EquipmentID)
	if err != nil {
		return nil, err
	}

	u, err := users.Get(ctx, client, req.UserID)
	if err != nil {
		return nil, err
	}

	e := &data.Exposure{
		ID:        uuid.New().String(),
		Equipment: *eq,
		Duration:  req.Duration,
		A8:        utils.PartialExposureA8(eq.VibrationMagnitude, req.Duration),
		Points:    utils.PartialExposurePoints(eq.VibrationMagnitude, req.Duration),
		User:      *u,
		CreatedAt: time.Now().UTC(),
	}

	if err := client.CreateExposure(ctx, e); err != nil {
		return nil, err
	}

	_ = pub.Publish(ctx, "exposure.recorded", e)
	publishThresholdEvents(ctx, client, pub, u.ID, e.CreatedAt)

	return e, nil
}

// Get retrieves a single exposure by ID.
func Get(ctx context.Context, client db.Client, id string) (*data.Exposure, error) {
	return client.GetExposure(ctx, id)
}

// List returns all exposure records.
func List(ctx context.Context, client db.Client) ([]*data.Exposure, error) {
	return client.ListExposures(ctx)
}

// GetSummary aggregates a user's exposures within the given time window.
// Points are summed additively; A(8) is combined using root-sum-of-squares
// per the HSE multi-source exposure method.
func GetSummary(ctx context.Context, client db.Client, userID string, start, end *time.Time) (*data.ExposureSummary, error) {
	u, err := users.Get(ctx, client, userID)
	if err != nil {
		return nil, err
	}

	exposures, err := client.GetExposuresByUser(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	var totalPoints, sumA8Squared float64
	for _, ex := range exposures {
		totalPoints += ex.Points
		sumA8Squared += ex.A8 * ex.A8
	}

	return &data.ExposureSummary{
		A8:     math.Sqrt(sumA8Squared),
		Points: totalPoints,
		User:   *u,
	}, nil
}

// publishThresholdEvents checks the user's daily cumulative exposure after a new
// record is added and publishes EAV or ELV events when HSE thresholds are crossed.
func publishThresholdEvents(ctx context.Context, client db.Client, pub Publisher, userID string, recordedAt time.Time) {
	dayStart := recordedAt.Truncate(24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)

	summary, err := GetSummary(ctx, client, userID, &dayStart, &dayEnd)
	if err != nil {
		return
	}

	switch {
	case summary.A8 >= thresholdELVA8 || summary.Points >= thresholdELVPoints:
		_ = pub.Publish(ctx, "exposure.elv_reached", summary)
	case summary.A8 >= thresholdEAVA8 || summary.Points >= thresholdEAVPoints:
		_ = pub.Publish(ctx, "exposure.eav_reached", summary)
	}
}
