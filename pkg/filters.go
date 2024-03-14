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
		var index string
		if (filterType == "dataUseRegister") {
			index = strings.ToLower(filterType)
		} else if (filterType == "paper") {
			index = "publication"
		} else {
			index = filterType
		}

		filterKey, ok := filter["keys"].(string)
		if !ok {
			log.Printf("Filter keys in %s not recognised", filter)
		}

		response, err := ElasticClient.Search(
			ElasticClient.Search.WithIndex(index),
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

		if (filterKey == "dateRange") || (filterKey == "publicationDate") {
			startValue := elasticResp.Aggregations["startDate"].(map[string]interface{})["value_as_string"]
			endValue := elasticResp.Aggregations["endDate"].(map[string]interface{})["value_as_string"]
			allFilters = append(allFilters, gin.H{
				filterType: gin.H{
					filterKey: gin.H{
						"buckets": []gin.H{
							{
								"key": "startDate",
								"value": startValue,
							},
							{	
								"key": "endDate",
								"value": endValue,
							},
						},
					},
				},
			})
		} else {
			allFilters = append(allFilters, gin.H{filterType: elasticResp.Aggregations})
		}
	}

	c.JSON(http.StatusOK, gin.H{"filters": allFilters})
}

func filtersRequest(filter map[string]interface{}) gin.H {
	filterKey, ok := filter["keys"].(string)
	var aggs gin.H
	if !ok {
		log.Printf("Filter key in %s not recognised", filter["keys"])
	}
	if (filterKey == "dateRange") {
		aggs = gin.H{
			"size": 0,
			"aggs": gin.H{
				"startDate": gin.H{
					"min": gin.H{"field": "startDate"},
				},
				"endDate": gin.H{
					"max": gin.H{"field": "endDate"},
				},
			},
		}
	} else if (filterKey == "publicationDate") {
		aggs = gin.H{
			"size": 0,
			"aggs": gin.H{
				"startDate": gin.H{
					"min": gin.H{"field": "publicationDate"},
				},
				"endDate": gin.H{
					"max": gin.H{"field": "publicationDate"},
				},
			},
		}
	} else if (filterKey == "populationSize") {
		ranges := populationRanges()
		aggs = gin.H{
			"size":0,
			"aggs": gin.H{
				"populationSize": gin.H{
					"range": gin.H{"field": filterKey, "ranges": ranges},
				},
			},
		}
	} else {
		aggs = gin.H{
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
	return aggs
}