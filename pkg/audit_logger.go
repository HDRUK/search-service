package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
	"github.com/gin-gonic/gin"
)

func pubSubAudit(actionType string, actionName string, description string) {
	auditEnabled := os.Getenv("AUDIT_LOG_ENABLED")

	if (auditEnabled == "true") {
		ctx := context.Background()
		projectId := os.Getenv("PUBSUB_PROJECT_ID")
		topicName := os.Getenv("PUBSUB_TOPIC_NAME")

		client, err := pubsub.NewClient(ctx, projectId)
		if err != nil {
			log.Printf("Failed to create client: %v", err)
		}
		defer client.Close()

		messageJson := gin.H{
			"action_type": actionType,
			"action_name": actionName,
			"action_service": "search-service",
			"description": description,
		}
		messageByte, err := json.Marshal(messageJson)
		if err != nil {
			log.Print(err.Error())
		}

		message := &pubsub.Message{Data: messageByte}

		topic := client.Topic(topicName)
		res := topic.Publish(ctx, message)

		id, err := res.Get(ctx)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Printf("\npublisher response: %s\n", id)

		fmt.Println("message published")
	}
}