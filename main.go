package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	dbmongo "github.com/ctrl-hub/challenge/db/mongo"
	"github.com/ctrl-hub/challenge/exposure"
	"github.com/ctrl-hub/challenge/handler"
)

func main() {
	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	port := getEnv("PORT", "8080")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := dbmongo.New(ctx, mongoURI)
	if err != nil {
		log.Fatalf("connecting to MongoDB: %v", err)
	}
	log.Println("Connected to MongoDB")

	client.Seed(context.Background())

	h := handler.New(client, exposure.NoopPublisher{})

	addr := ":" + port
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, h.Routes()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
