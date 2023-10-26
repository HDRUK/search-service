package elastic

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"github.com/elastic/go-elasticsearch/v8"
)

// Defines the ElasticSearch client, authentication and elastic deployment
// endpoint are required environment variables.
func DefaultClient() *elasticsearch.Client {
	// Note: we might not need to define custom transport with infra hosted elastic
	// It is defined here in order to disable SSL cert verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	clusterURLs := []string{os.Getenv("ELASTIC_URL")}
	username := os.Getenv("ELASTIC_USERNAME")
	password := os.Getenv("ELASTIC_PASSWORD")

	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: clusterURLs,
		Username:  username,
		Password:  password,
		Transport: tr,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
	return es
}
