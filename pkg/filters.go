package search

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Aggregations struct {
	Took	int	`json:"took"`
	TimedOut	bool	`json:"timed_out"`
	Aggregations	map[string]interface{}	`json:"aggregations"`
}

func ListDatasetFilters(c *gin.Context) {
	var buf bytes.Buffer

	elasticQuery := datasetFiltersRequest()
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

	c.JSON(http.StatusOK, elasticResp)
}

func datasetFiltersRequest() gin.H {
	return gin.H{
		"size": 0,
		"aggs": gin.H{
			"publisherName": gin.H{
				"terms": gin.H{
					"field": "publisherName", 
					"size":100,
			  	},
			},
		},
	}
}