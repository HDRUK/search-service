package search

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"hdruk/search-service/utils/elastic"
)

// Loads environment variables and defines the ElasticSearch client
// Authentication and elastic deployment endpoint are required environment variables
// func init() {
// 	err := godotenv.Load("../.env")
// 	if err != nil {
// 		log.Fatal(err.Error())
// 	}

// 	// Note: we might not need to define custom transport with infra hosted elastic
// 	// It is defined here in order to disable SSL cert verification
// 	tr := &http.Transport{
//         TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
//     }
// 	clusterURLs := []string{os.Getenv("ELASTIC_URL")}
// 	username := os.Getenv("ELASTIC_USERNAME")
// 	password := os.Getenv("ELASTIC_PASSWORD")

// 	es, err := elasticsearch.NewClient(elasticsearch.Config{
// 		Addresses: clusterURLs,
// 		Username: username,
// 		Password:  password,
// 		Transport: tr,
// 	})
// 	if err != nil {
// 		log.Fatal(err.Error())
// 	}
// 	elasticClient = es
// }

// Query represents the search query incoming from the gateway-api
// Expected to be a string in the body of the request e.g. {'query': 'diabetes snomed'}
type Query struct {
	QueryString string `json:"query"`
}

// SearchResponse represents the expected structure of results returned by ElasticSearch
type SearchResponse struct {
	Took int `json:"took"`
	TimedOut bool `json:"timed_out"`
	Shards map[string]interface{} `json:"_shards"`
	Hits map[string]interface{} `json:"hits"`
}

// SearchGeneric performs searches of the ElasticSearch indices for datasets,
// tools and collections, using the query supplied in the gin.Context.
// Search results are returned grouped by entity type.
func SearchGeneric(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
        return
    }
	datasetResults := make(chan SearchResponse)
	toolResults := make(chan SearchResponse)
	collectionResults := make(chan SearchResponse)

	go func() {
        datasetSearch(query, datasetResults)
    }()
    go func() {
        toolSearch(query, toolResults)
    }()
	go func() {
        collectionSearch(query, collectionResults)
    }()

	for i := 0; i < 3; i++ {
        select {
        case datasets := <-datasetResults:
            c.JSON(http.StatusOK, gin.H{"datasets": datasets})
        case tools := <-toolResults:
            c.JSON(http.StatusOK, gin.H{"tools": tools})
		case collections := <-collectionResults:
			c.JSON(http.StatusOK, gin.H{"collections": collections})
		}
    }
}

// datasetSearch performs a search of the ElasticSearch datasets index using 
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func datasetSearch(query Query, res chan SearchResponse) {
	//elasticClient := newElasticClient()

	var buf bytes.Buffer
	// TO DO: update with elastic config for datasets
	elasticQuery := gin.H{
		"query": gin.H{
			"multi_match": gin.H{
		  		"query": query.QueryString,
			},
	  	},
	}
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
	  log.Fatal(err.Error())
	}

	response, err := elastic.ElasticClient.Search(
		elastic.ElasticClient.Search.WithIndex("dracula"),
		elastic.ElasticClient.Search.WithBody(&buf),
	)
	
	if err != nil {
		log.Fatal(err.Error())
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var elasticResp SearchResponse
	json.Unmarshal(body, &elasticResp)

	res <- elasticResp
}

// toolSearch performs a search of the ElasticSearch tools index using 
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func toolSearch(query Query, res chan SearchResponse) {
	stubResp := SearchResponse{1, false, gin.H{}, gin.H{}}
	res <- stubResp
}

// collectionsSearch performs a search of the ElasticSearch collections index using 
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func collectionSearch(query Query, res chan SearchResponse) {
	stubResp := SearchResponse{1, false, gin.H{}, gin.H{}}
	res <- stubResp
}
