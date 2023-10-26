package elastic

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/joho/godotenv"
)

var (
	ElasticClient *elasticsearch.Client
)

// Loads environment variables and defines the ElasticSearch client
// Authentication and elastic deployment endpoint are required environment variables
func init() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal(err.Error())
	}

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
	ElasticClient = es
}
