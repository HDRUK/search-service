package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type Aggregations struct {
	Took         int                    `json:"took"`
	TimedOut     bool                   `json:"timed_out"`
	Aggregations map[string]interface{} `json:"aggregations"`
}

type FilterRequest struct {
	Filters []map[string]interface{} `json:"filters"`
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
		slog.Warn(fmt.Sprintf("Could not bind filter request: %s", c.Request.Body))
	}

	type result struct {
		index int
		entry gin.H
	}

	resultCh := make(chan result, len(filterRequest.Filters))
	var wg sync.WaitGroup

	for i, filter := range filterRequest.Filters {
		wg.Add(1)
		go func(i int, filter map[string]interface{}) {
			defer wg.Done()
			entry := queryFilter(filter)
			if entry != nil {
				resultCh <- result{index: i, entry: entry}
			}
		}(i, filter)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results preserving insertion order.
	ordered := make([]gin.H, len(filterRequest.Filters))
	for r := range resultCh {
		ordered[r.index] = r.entry
	}
	allFilters := []gin.H{}
	for _, entry := range ordered {
		if entry != nil {
			allFilters = append(allFilters, entry)
		}
	}

	c.JSON(http.StatusOK, gin.H{"filters": allFilters})
}

// queryFilter executes a single filter aggregation query against Elastic and
// returns the formatted gin.H entry for inclusion in the ListFilters response.
// Extracting this into its own function ensures response.Body is closed after
// each query rather than accumulating defers until ListFilters returns.
func queryFilter(filter map[string]interface{}) gin.H {
	filterType, ok := filter["type"].(string)
	if !ok {
		slog.Debug(fmt.Sprintf("Filter type in %v not recognised", filter))
		return nil
	}

	filterKey, ok := filter["keys"].(string)
	if !ok {
		slog.Debug(fmt.Sprintf("Filter keys in %v not recognised", filter))
		return nil
	}

	var index string
	if filterType == "dataUseRegister" || filterType == "dataProvider" {
		index = strings.ToLower(filterType)
	} else if filterType == "paper" {
		index = "publication"
	} else {
		index = filterType
	}

	var buf bytes.Buffer
	elasticQuery := filtersRequest(filter)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		slog.Info(fmt.Sprintf("Failed to encode filters request: %s", err.Error()))
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex(index),
		ElasticClient.Search.WithBody(&buf),
	)
	if err != nil {
		slog.Warn(err.Error())
		return nil
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Warn(err.Error())
		return nil
	}

	var elasticResp SearchResponse
	if err := json.Unmarshal(body, &elasticResp); err != nil {
		slog.Warn(fmt.Sprintf("Failed to unmarshal filter response: %s", err.Error()))
	}

	if len(elasticResp.Aggregations) == 0 {
		slog.Warn(fmt.Sprintf("No aggregations returned for filter: %s - %s", filterType, filterKey))
	}

	if filterKey == "dateRange" || filterKey == "publicationDate" {
		startAgg, ok := elasticResp.Aggregations["startDate"].(map[string]interface{})
		if !ok {
			slog.Warn(fmt.Sprintf("Unexpected startDate aggregation format for filter: %s - %s", filterType, filterKey))
			return nil
		}
		endAgg, ok := elasticResp.Aggregations["endDate"].(map[string]interface{})
		if !ok {
			slog.Warn(fmt.Sprintf("Unexpected endDate aggregation format for filter: %s - %s", filterType, filterKey))
			return nil
		}
		return gin.H{
			filterType: gin.H{
				filterKey: gin.H{
					"buckets": []gin.H{
						{"key": "startDate", "value": startAgg["value_as_string"]},
						{"key": "endDate", "value": endAgg["value_as_string"]},
					},
				},
			},
		}
	}

	return gin.H{filterType: elasticResp.Aggregations}
}

func filtersRequest(filter map[string]interface{}) gin.H {
	filterKey, ok := filter["keys"].(string)
	var aggs gin.H
	if !ok {
		slog.Info(fmt.Sprintf("Filter key in %s not recognised", filter["keys"]))
	}
	if filterKey == "dateRange" {
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
	} else if filterKey == "publicationDate" {
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
	} else if filterKey == "populationSize" {
		ranges := populationRangesCache
		aggs = gin.H{
			"size": 0,
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
				filter["keys"].(string): gin.H{
					"terms": gin.H{
						"field": filter["keys"].(string),
						"size":  1000,
					},
				},
			},
		}
	}
	return aggs
}
