package equipment

import (
	"context"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/db"
)

func Get(ctx context.Context, client db.Client, id string) (*data.EquipmentItem, error) {
	return client.GetEquipment(ctx, id)
}

func List(ctx context.Context, client db.Client) ([]*data.EquipmentItem, error) {
	return client.ListEquipment(ctx)
}
