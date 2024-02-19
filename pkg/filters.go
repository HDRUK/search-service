package search

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Aggregations struct {
	Took	int	`json:"took"`
	TimedOut	bool	`json:"timed_out"`
	Aggregations	map[string]interface{}	`json:"aggregations"`
}

type FilterRequest struct {
	Filters	[]map[string]interface{} `json:"filters"`
}

/*
ListFilters lists all the values available for the filter type and key pairs
in the given FilterRequest.
The `type` must match an existing elasticsearch index.
The `keys` must match a field name in that index.
The expected structure of a FilterRequest is:

```
{
	"filters": [
		{
			"type": "dataset",
			"keys": "publisherName"
		},
		{
			"type": "dataset",
			"keys": "containsTissue"
		}
	]
}
```
*/
func ListFilters(c *gin.Context) {
	var filterRequest FilterRequest
	if err := c.BindJSON(&filterRequest); err != nil {
		log.Printf("Could not bind filter request: %s", c.Request.Body)
	}

	var allFilters []gin.H

	for _, filter := range(filterRequest.Filters) {
		var buf bytes.Buffer
		elasticQuery := filtersRequest(filter)
		if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
			log.Fatal(err.Error())
		}

		filterType, ok := filter["type"].(string)
		if !ok {
			log.Printf("Filter type in %s not recognised", filter)
		}

		response, err := ElasticClient.Search(
			ElasticClient.Search.WithIndex(strings.ToLower(filterType)),
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

		allFilters = append(allFilters, gin.H{filterType: elasticResp.Aggregations})
	}

	c.JSON(http.StatusOK, gin.H{"filters": allFilters})
}

func filtersRequest(filter map[string]interface{}) gin.H {
	return gin.H{
		"size": 0,
		"aggs": gin.H{
			filter["keys"].(string) : gin.H{
				"terms": gin.H{
					"field": filter["keys"].(string), 
					"size":1000,
			  	},
			},
		},
	}
}