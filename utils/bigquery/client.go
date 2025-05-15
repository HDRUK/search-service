package bigqueryclient

import (
	"context"
	"log"
	"os"

	"cloud.google.com/go/bigquery"
)

func DefaultBigQueryClient() *bigquery.Client {
	projectID := os.Getenv("BQ_PROJECT_ID")
	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err.Error())
	}
	return client
}

