package data

import "time"

type User struct {
	ID   string `json:"id"   bson:"_id"`
	Name string `json:"name" bson:"name"`
}

type EquipmentItem struct {
	ID                 string  `json:"id"                  bson:"_id"`
	Name               string  `json:"name"                bson:"name"`
	VibrationMagnitude float64 `json:"vibration_magnitude" bson:"vibration_magnitude"`
}

type Exposure struct {
	ID        string        `json:"id"         bson:"_id"`
	Equipment EquipmentItem `json:"equipment"  bson:"equipment"`
	Duration  int           `json:"duration"   bson:"duration"`
	A8        float64       `json:"a8"         bson:"a8"`
	Points    float64       `json:"points"     bson:"points"`
	User      User          `json:"user"       bson:"user"`
	CreatedAt time.Time     `json:"created_at" bson:"created_at"`
}

type ExposureSummary struct {
	A8     float64 `json:"a8"`
	Points float64 `json:"points"`
	User   User    `json:"user"`
}

type RecordExposureRequest struct {
	EquipmentID string `json:"equipment_id"`
	Duration    int    `json:"duration"`
	UserID      string `json:"user_id"`
}
