package search

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"

	"hdruk/search-service/utils/elastic"
)

var (
	ElasticClient *elasticsearch.Client
)

func DefineElasticClient() {
	ElasticClient = elastic.DefaultClient()
}

// Query represents the search query incoming from the gateway-api
// Expected to be a string in the body of the request e.g. {'query': 'diabetes snomed'}
type Query struct {
	QueryString string `json:"query"`
}

// SearchResponse represents the expected structure of results returned by ElasticSearch
type SearchResponse struct {
	Took     int                    `json:"took"`
	TimedOut bool                   `json:"timed_out"`
	Shards   map[string]interface{} `json:"_shards"`
	Hits     map[string]interface{} `json:"hits"`
}

// SearchGeneric performs searches of the ElasticSearch indices for datasets,
// tools and collections, using the query supplied in the gin.Context.
// Search results are returned grouped by entity type.
func SearchGeneric(c *gin.Context) {
	var query Query

	if err := c.BindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return 
	}
	datasetResults := make(chan SearchResponse)
	toolResults := make(chan SearchResponse)
	collectionResults := make(chan SearchResponse)

	results := make(map[string]interface{})
	

	go datasetSearch(query, datasetResults)
	go toolSearch(query, toolResults)
	go collectionSearch(query, collectionResults)

	for i := 0; i < 3; i++ {
		select {
		case datasets := <-datasetResults:
			results["datasets"] = datasets
		case tools := <-toolResults:
			results["tools"] = tools
		case collections := <-collectionResults:
			results["collections"] = collections
		}
	}

	c.JSON(http.StatusOK, results)
}

// datasetSearch performs a search of the ElasticSearch datasets index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func datasetSearch(query Query, res chan SearchResponse) {
	var buf bytes.Buffer

	elasticQuery := datasetElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		log.Fatal(err.Error())
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("datasets"),
		ElasticClient.Search.WithBody(&buf),
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

// datasetElasticConfig defines the body of the query to the elastic datasets index
func datasetElasticConfig(query Query) gin.H {
	searchableFields := []string{
		"abstract",
		"keywords",
		"description",
		"shortTitle",
		"title",
		"publisher_name",
		"named_entities",
	}
	mm1 := gin.H{
		"multi_match": gin.H{
			"query":     query.QueryString,
			"fields":    searchableFields,
			"fuzziness": "AUTO:5,7",
			"analyzer": "medterms_analyzer",
		},
	}
	mm2 := gin.H{
		"multi_match": gin.H{
			"query":     query.QueryString,
			"fields":    searchableFields,
			"fuzziness": "AUTO:5,7",
			"analyzer": "medterms_analyzer",
			"operator":  "and",
		},
	}
	mm3 := gin.H{
		"multi_match": gin.H{
			"query":  query.QueryString,
			"type":   "phrase",
			"fields": searchableFields,
			"analyzer": "medterms_analyzer",
			"boost":  2,
		},
	}

	return gin.H{
		"size": 1000,
		"query": gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		},
		"highlight": gin.H{
			"fields": gin.H{
				"description": gin.H{},
				"abstract":       gin.H{},
			},
		},
		"explain": true,
	}
}

// toolSearch performs a search of the ElasticSearch tools index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func toolSearch(query Query, res chan SearchResponse) {
	var buf bytes.Buffer

	elasticQuery := toolsElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		log.Fatal(err.Error())
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("tools"),
		ElasticClient.Search.WithBody(&buf),
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

// toolsElasticConfig defines the body of the query to the elastic tools index
func toolsElasticConfig(query Query) gin.H {
	searchableFields := []string{
		"tags",
		"programmingLanguage",
		"name",
		"link",
		"description",
		"resultsInsights",
		"license",
	}
	mm1 := gin.H{
		"multi_match": gin.H{
			"query":     query.QueryString,
			"fields":    searchableFields,
			"fuzziness": "AUTO:5,7",
		},
	}
	mm2 := gin.H{
		"multi_match": gin.H{
			"query":     query.QueryString,
			"fields":    searchableFields,
			"fuzziness": "AUTO:5,7",
			"operator":  "and",
		},
	}
	mm3 := gin.H{
		"multi_match": gin.H{
			"query":  query.QueryString,
			"fields": searchableFields,
			"type":   "phrase",
			"boost":  2,
		},
	}
	return gin.H{
		"size": 500,
		"query": gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		},
		"highlight": gin.H{
			"fields": gin.H{
				"name":        gin.H{},
				"description": gin.H{},
			},
		},
		"explain": true,
	}
}

// collectionsSearch performs a search of the ElasticSearch collections index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func collectionSearch(query Query, res chan SearchResponse) {
	var buf bytes.Buffer

	elasticQuery := collectionsElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		log.Fatal(err.Error())
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("collections"),
		ElasticClient.Search.WithBody(&buf),
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

// collectionsElasticConfig defines the body of the query to the elastic collections index
func collectionsElasticConfig(query Query) gin.H {
	relatedObjectFields := []string{
		"relatedObjects.keywords",
		"relatedObjects.title",
		"relatedObjects.name",
		"relatedObjects.description",
	}
	searchableFields := []string{
		"description",
		"name",
		"keywords",
	}
	mm1 := gin.H{
		"multi_match": gin.H{
			"query":     query.QueryString,
			"fields":    relatedObjectFields,
			"fuzziness": "AUTO:5,7",
		},
	}
	mm2 := gin.H{
		"multi_match": gin.H{
			"query":     query.QueryString,
			"fields":    searchableFields,
			"fuzziness": "AUTO:5,7",
			"boost":     2,
		},
	}
	mm3 := gin.H{
		"multi_match": gin.H{
			"query":  query.QueryString,
			"type":   "phrase",
			"fields": searchableFields,
			"boost":  3,
		},
	}
	return gin.H{
		"size": 120,
		"query": gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		},
		"highlight": gin.H{
			"fields": gin.H{
				"description": gin.H{},
				"name":        gin.H{},
				"keywords":    gin.H{},
			},
		},
		"explain": true,
	}
}
