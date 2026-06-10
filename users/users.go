package users

import (
	"context"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/db"
)

func Get(ctx context.Context, client db.Client, id string) (*data.User, error) {
	return client.GetUser(ctx, id)
}
