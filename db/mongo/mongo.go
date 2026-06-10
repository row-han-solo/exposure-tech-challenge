package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	gomongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ctrl-hub/challenge/data"
	"github.com/ctrl-hub/challenge/db"
)

const (
	dbName       = "havs"
	colUsers     = "users"
	colEquipment = "equipment"
	colExposures = "exposures"
)

// Client is a MongoDB-backed implementation of db.Client.
type Client struct {
	mdb *gomongo.Database
}

// New connects to MongoDB at the given URI and returns a ready Client.
func New(ctx context.Context, uri string) (*Client, error) {
	mc, err := gomongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := mc.Ping(ctx, nil); err != nil {
		return nil, err
	}
	c := &Client{mdb: mc.Database(dbName)}
	c.ensureIndexes(ctx)
	return c, nil
}

func (c *Client) ensureIndexes(ctx context.Context) {
	c.mdb.Collection(colExposures).Indexes().CreateOne(ctx, gomongo.IndexModel{
		Keys: bson.D{
			{Key: "user._id", Value: 1},
			{Key: "created_at", Value: 1},
		},
	})
}

func (c *Client) GetUser(ctx context.Context, id string) (*data.User, error) {
	var u data.User
	err := c.mdb.Collection(colUsers).FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if errors.Is(err, gomongo.ErrNoDocuments) {
		return nil, db.ErrNotFound
	}
	return &u, err
}

func (c *Client) GetEquipment(ctx context.Context, id string) (*data.EquipmentItem, error) {
	var eq data.EquipmentItem
	err := c.mdb.Collection(colEquipment).FindOne(ctx, bson.M{"_id": id}).Decode(&eq)
	if errors.Is(err, gomongo.ErrNoDocuments) {
		return nil, db.ErrNotFound
	}
	return &eq, err
}

func (c *Client) ListEquipment(ctx context.Context) ([]*data.EquipmentItem, error) {
	cursor, err := c.mdb.Collection(colEquipment).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var items []*data.EquipmentItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) CreateExposure(ctx context.Context, e *data.Exposure) error {
	_, err := c.mdb.Collection(colExposures).InsertOne(ctx, e)
	return err
}

func (c *Client) GetExposure(ctx context.Context, id string) (*data.Exposure, error) {
	var e data.Exposure
	err := c.mdb.Collection(colExposures).FindOne(ctx, bson.M{"_id": id}).Decode(&e)
	if errors.Is(err, gomongo.ErrNoDocuments) {
		return nil, db.ErrNotFound
	}
	return &e, err
}

func (c *Client) ListExposures(ctx context.Context) ([]*data.Exposure, error) {
	cursor, err := c.mdb.Collection(colExposures).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var items []*data.Exposure
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) GetExposuresByUser(ctx context.Context, userID string, start, end *time.Time) ([]*data.Exposure, error) {
	filter := bson.M{"user._id": userID}
	if start != nil || end != nil {
		timeFilter := bson.M{}
		if start != nil {
			timeFilter["$gte"] = *start
		}
		if end != nil {
			timeFilter["$lte"] = *end
		}
		filter["created_at"] = timeFilter
	}

	cursor, err := c.mdb.Collection(colExposures).Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var items []*data.Exposure
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}
