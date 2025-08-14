package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
)

type LogEntry struct {
	Data        string            `json:"data"`
	Attributes  map[string]string `json:"attributes"`
	MessageId   string            `json:"messageId"`
	PublishTime time.Time         `json:"publishTime"`
}

func main() {
	f, err := os.OpenFile("logs.json", os.O_RDONLY, 0o644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		fmt.Println("Please create one by running the following command:")
		fmt.Println("gcloud pubsub subscriptions pull --project $PROJECT $SUBSCRIPTION --format json --limit 10 > logs.json")
		os.Exit(1)
	}
	defer f.Close()
	os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:3004")

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, "nais-local-dev")
	if err != nil {
		fmt.Println("Error creating Pub/Sub client:", err)
		os.Exit(1)
	}

	topic := client.Topic("nais-api-log-topic")

	entries := make([]struct {
		Message LogEntry `json:"message"`
	}, 0)
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		fmt.Println("Error decoding JSON:", err)
		os.Exit(1)
	}

	for _, entry := range entries {
		msg := pubsub.Message{
			Data:        []byte(entry.Message.Data),
			Attributes:  entry.Message.Attributes,
			ID:          entry.Message.MessageId,
			PublishTime: entry.Message.PublishTime,
		}

		fmt.Println(entry)

		result := topic.Publish(ctx, &msg)
		id, err := result.Get(ctx)
		if err != nil {
			fmt.Println("Error publishing message:", err)
			continue
		}
		fmt.Printf("Published message with ID: %s\n", id)
	}
}
