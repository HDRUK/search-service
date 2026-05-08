package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gin-gonic/gin"
)

// pubsubClient is initialised once at startup when audit logging is enabled.
// Creating a new client per call would open a new gRPC connection each time.
var pubsubClient *pubsub.Client

// InitAuditLogger creates the PubSub client singleton. Must be called after
// godotenv.Load() in main so that env vars are available.
func InitAuditLogger() {
	if os.Getenv("AUDIT_LOG_ENABLED") != "true" {
		return
	}
	ctx := context.Background()
	var err error
	pubsubClient, err = pubsub.NewClient(ctx, os.Getenv("PUBSUB_PROJECT_ID"))
	if err != nil {
		log.Printf("Failed to create pubsub client: %v", err)
	}
}

func pubSubAudit(actionType string, actionName string, description string) {
	if os.Getenv("AUDIT_LOG_ENABLED") != "true" || pubsubClient == nil {
		return
	}

	ctx := context.Background()
	topicName := os.Getenv("PUBSUB_TOPIC_NAME")
	serviceName := os.Getenv("PUBSUB_SERVICE_NAME")

	messageJson := gin.H{
		"action_type":    actionType,
		"action_name":    actionName,
		"action_service": serviceName,
		"description":    description,
		"created_at":     time.Now().UnixMicro(),
	}
	messageByte, err := json.Marshal(messageJson)
	if err != nil {
		log.Print(err.Error())
		return
	}

	topic := pubsubClient.Topic(topicName)
	res := topic.Publish(ctx, &pubsub.Message{Data: messageByte})

	id, err := res.Get(ctx)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("\npublisher response: %s\n", id)
	fmt.Println("message published")
}
