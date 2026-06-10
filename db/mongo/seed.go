package mongo

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ctrl-hub/challenge/data"
)

var seedUsers = []data.User{
	{ID: "713be58e-0d79-4df2-a85c-9f44ca513a7d", Name: "Alice Smith"},
	{ID: "b2e8c3a1-5f7d-4a9e-8c2b-1d3e6f8a0b4c", Name: "Bob Jones"},
}

var seedEquipment = []data.EquipmentItem{
	{
		ID:                 "2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49",
		Name:               "AirCat - Drill - 4337",
		VibrationMagnitude: 2.1,
	},
	{
		ID:                 "36603447-2f30-41b1-a908-526c0b6f1755",
		Name:               "JCB - Hydraulic Breaker - CEJCBHM25",
		VibrationMagnitude: 4.0,
	},
}

// Seed upserts the default users and equipment items into the database.
// It is idempotent — running it multiple times has no additional effect.
func (c *Client) Seed(ctx context.Context) {
	for _, u := range seedUsers {
		u := u
		if _, err := c.mdb.Collection(colUsers).UpdateOne(
			ctx,
			bson.M{"_id": u.ID},
			bson.M{"$setOnInsert": u},
			options.Update().SetUpsert(true),
		); err != nil {
			log.Printf("seed user %s: %v", u.ID, err)
		}
	}

	for _, eq := range seedEquipment {
		eq := eq
		if _, err := c.mdb.Collection(colEquipment).UpdateOne(
			ctx,
			bson.M{"_id": eq.ID},
			bson.M{"$setOnInsert": eq},
			options.Update().SetUpsert(true),
		); err != nil {
			log.Printf("seed equipment %s: %v", eq.ID, err)
		}
	}

	log.Println("Seed data ready")
	log.Println("  Users:")
	for _, u := range seedUsers {
		log.Printf("    %s  %s", u.ID, u.Name)
	}
	log.Println("  Equipment:")
	for _, eq := range seedEquipment {
		log.Printf("    %s  %s  (%.1f m/s²)", eq.ID, eq.Name, eq.VibrationMagnitude)
	}
}
