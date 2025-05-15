package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"reflect"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/googleapi"

	bigqueryclient "hdruk/search-service/utils/bigquery"
	"hdruk/search-service/utils/elastic"
)

var (
	ElasticClient  *elasticsearch.Client
	BigQueryClient *bigquery.Client
	BQUpload       = uploadSearchAnalytics
)

func DefineElasticClient() {
	ElasticClient = elastic.DefaultClient()
	BigQueryClient = bigqueryclient.DefaultBigQueryClient()
}

/*
	Query represents the search query incoming from the gateway-api

The body of the request is expected to have the following structure
```

	{
		"query": <query_term>,
		"filters": {
			<type>: {
				<key>: [
					<value1>,
					...
				]
			}
		}
	}

```
where:
- query_term is a string e.g. "asthma"
- type is a string matching the name of an elasticsearch index e.g. "dataset"
- key is a string matching a field in the elastic search index specified e.g. "publisherName"
- value1 is a value matching values in the specified fields of the elastic index e.g. "publisher A"
*/
type Query struct {
	QueryString  string                            `json:"query"`
	Filters      map[string]map[string]interface{} `json:"filters"`
	Aggregations []map[string]interface{}          `json:"aggs"`
	IDs          []string                          `json:"ids"`
}

type SimilarSearch struct {
	ID string `json:"id"`
}

// SearchResponse represents the expected structure of results returned by ElasticSearch
type SearchResponse struct {
	Took         int                    `json:"took"`
	TimedOut     bool                   `json:"timed_out"`
	Shards       map[string]interface{} `json:"_shards"`
	Hits         HitsField              `json:"hits"`
	Aggregations map[string]interface{} `json:"aggregations"`
}

type HitsField struct {
	Total    map[string]interface{} `json:"total"`
	MaxScore float64                `json:"max_score"`
	Hits     []Hit                  `json:"hits"`
}

type Hit struct {
	Explanation map[string]interface{} `json:"_explanation"`
	Id          string                 `json:"_id"`
	Score       float64                `json:"_score"`
	Source      map[string]interface{} `json:"_source"`
	Highlight   map[string][]string    `json:"highlight"`
}

type SearchErrorResponse struct {
	Error  map[string][]RootCause `json:"error"`
	Status int                    `json:"status"`
}

type RootCause struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
	Index  string `json:"index"`
}

type SearchAnalytics struct {
	Timestamp        string
	EntityType       string
	SearchTerm       string
	FilterUsed       string
	PageResults      string
	EntitiesReturned int
}

func (a *SearchAnalytics) Save() (map[string]bigquery.Value, string, error) {
	return map[string]bigquery.Value{
		"Timestamp":        a.Timestamp,
		"SearchTerm":       a.SearchTerm,
		"FilterUsed":       a.FilterUsed,
		"PageResults":      a.PageResults,
		"EntitiesReturned": a.EntitiesReturned,
	}, "", nil
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
	dataUseResults := make(chan SearchResponse)
	publicationResults := make(chan SearchResponse)
	dataProviderResults := make(chan SearchResponse)
	dataCustodianNetworkResults := make(chan SearchResponse)

	results := make(map[string]interface{})

	go datasetChannel(query, datasetResults)
	go toolChannel(query, toolResults)
	go collectionChannel(query, collectionResults)
	go dataUseChannel(query, dataUseResults)
	go publicationChannel(query, publicationResults)
	go dataProviderChannel(query, dataProviderResults)
	go dataCustodianNetworkChannel(query, dataCustodianNetworkResults)

	for i := 0; i < 7; i++ {
		select {
		case datasets := <-datasetResults:
			results["dataset"] = datasets
		case tools := <-toolResults:
			results["tool"] = tools
		case collections := <-collectionResults:
			results["collection"] = collections
		case data_uses := <-dataUseResults:
			results["dataUseRegister"] = data_uses
		case publications := <-publicationResults:
			results["publication"] = publications
		case dataProviders := <-dataProviderResults:
			results["dataProvider"] = dataProviders
		case dataCustodianNetworks := <-dataCustodianNetworkResults:
			results["datacustodiannetwork"] = dataCustodianNetworks
		}
	}

	c.JSON(http.StatusOK, results)
}

func DatasetSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		slog.Debug(fmt.Sprintf("Failed to interpret search query with %s", err.Error()))
		return
	}

	results := datasetSearch(query)
	BQUpload(query, results, "dataset")
	c.JSON(http.StatusOK, results)
}

func datasetChannel(query Query, res chan SearchResponse) {
	elasticResp := datasetSearch(query)
	res <- elasticResp
}

// datasetSearch performs a search of the ElasticSearch datasets index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func datasetSearch(query Query) SearchResponse {
	var buf bytes.Buffer

	elasticQuery := datasetElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to encode elastic config %s with %s",
			elasticQuery,
			err.Error()),
		)
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("dataset"),
		ElasticClient.Search.WithBody(&buf),
	)

	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to execute elastic query with %s",
			err.Error()),
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to read elastic response with %s",
			err.Error()),
		)
	}

	var elasticResp SearchResponse
	json.Unmarshal(body, &elasticResp)

	if elasticResp.Hits.Hits == nil {
		// When there are genuinely no matches elastic returns hits == [].
		// When the hits == nil this implies something has actually gone wrong
		// for example an aggregation that cannot be calculated.
		// So we throw a warning in the case where hits == nil and more info
		// if debug mode is on.
		var elasticError SearchErrorResponse
		json.Unmarshal(body, &elasticError)
		// Try to extract the root cause message; if unable throw generic warning
		rootCause := elasticError.Error["root_cause"][0]
		if rootCause.Reason != "" {
			slog.Warn(
				fmt.Sprintf("Search query returned elastic error: %s",
					rootCause.Reason,
				))
		} else {
			slog.Warn("Hits from elastic are null, query may be malformed")
		}
		slog.Debug(fmt.Sprintf("Null result elastic query: %s", elasticQuery))
	}

	stripExplanation(elasticResp, query, "dataset")

	return elasticResp
}

// datasetElasticConfig defines the body of the query to the elastic datasets index
func datasetElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	var sortQuery []gin.H

	if query.QueryString == "" {
		if len(query.IDs) == 0 {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"match_all": gin.H{},
					},
					"random_score": gin.H{},
				},
			}
		} else {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"function_score": gin.H{
							"query": gin.H{
								"bool": gin.H{
									"filter": []gin.H{
										{
											"terms": gin.H{
												"_id": query.IDs,
											},
										},
									},
								},
							},
							"random_score": gin.H{},
						},
					},
				},
			}

			sortQuery = []gin.H{
				{
					"_script": gin.H{
						"type": "number",
						"script": gin.H{
							"lang":   "painless",
							"source": "params.order.indexOf(doc['_id'].value)",
							"params": gin.H{
								"order": query.IDs,
							},
						},
						"order": "asc",
					},
				},
			}
		}
	} else {
		searchableFields := []string{
			"abstract",
			"keywords",
			"description",
			"shortTitle",
			"title",
			"named_entities",
			"datasetDOI",
		}
		mm1 := gin.H{
			"multi_match": gin.H{
				"query":     query.QueryString,
				"fields":    searchableFields,
				"fuzziness": "AUTO:5,7",
				"analyzer":  "medterms_search_analyzer",
			},
		}
		mm2 := gin.H{
			"multi_match": gin.H{
				"query":     query.QueryString,
				"fields":    searchableFields,
				"fuzziness": "AUTO:5,7",
				"analyzer":  "medterms_search_analyzer",
				"operator":  "and",
				"boost":     2,
			},
		}
		mm3 := gin.H{
			"multi_match": gin.H{
				"query":    query.QueryString,
				"type":     "phrase",
				"fields":   searchableFields,
				"analyzer": "medterms_search_analyzer",
				"boost":    3,
			},
		}
		mainQuery = gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		}
	}

	mustFilters := []gin.H{}
	for key, terms := range query.Filters["dataset"] {
		filters := []gin.H{}
		if key == "dateRange" {
			rangeFilter := gin.H{
				"bool": gin.H{
					"must": []gin.H{
						{"range": gin.H{"startDate": gin.H{"lte": terms.([]interface{})[1]}}},
						{"range": gin.H{"endDate": gin.H{"gte": terms.([]interface{})[0]}}},
					},
				},
			}
			mustFilters = append(mustFilters, rangeFilter)
		} else if key == "populationSize" {
			includeNull := terms.(map[string]interface{})["includeUnreported"].(bool)
			from := terms.(map[string]interface{})["from"]
			to := terms.(map[string]interface{})["to"]
			var rangeFilter gin.H
			if includeNull {
				rangeFilter = gin.H{
					"bool": gin.H{
						"should": []gin.H{
							{"range": gin.H{key: gin.H{"gte": from, "lte": to}}},
							{"term": gin.H{"populationSize": -1}},
						},
					},
				}
			} else {
				rangeFilter = gin.H{
					"bool": gin.H{
						"must": []gin.H{
							{"range": gin.H{key: gin.H{"gte": from, "lte": to}}},
						},
					},
				}
			}
			mustFilters = append(mustFilters, rangeFilter)
		} else {
			for _, t := range terms.([]interface{}) {
				filters = append(filters, gin.H{"term": gin.H{key: t}})
			}
			mustFilters = append(mustFilters, gin.H{
				"bool": gin.H{
					"should": filters,
				},
			})
		}
	}

	f1 := gin.H{
		"bool": gin.H{
			"must": mustFilters,
		},
	}

	agg1 := buildAggregations(query)

	response := gin.H{
		"size":  os.Getenv("SEARCH_NO_RECORDS"),
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"description": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
				"abstract": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
			},
		},
		"explain":     true,
		"post_filter": f1,
		"aggs":        agg1,
	}

	if len(query.IDs) > 0 {
		response["sort"] = sortQuery
	}

	return response

}

func ToolSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		slog.Debug(fmt.Sprintf("Failed to interpret search query with %s", err.Error()))
		return
	}
	results := toolSearch(query)
	BQUpload(query, results, "tool")
	c.JSON(http.StatusOK, results)
}

func toolChannel(query Query, res chan SearchResponse) {
	elasticResp := toolSearch(query)
	res <- elasticResp
}

// toolSearch performs a search of the ElasticSearch tools index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func toolSearch(query Query) SearchResponse {
	var buf bytes.Buffer

	elasticQuery := toolsElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to encode elastic query %s with %s",
			elasticQuery,
			err.Error()),
		)
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("tool"),
		ElasticClient.Search.WithBody(&buf),
	)

	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to execute elastic query with %s",
			err.Error()),
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to read elastic response with %s",
			err.Error()),
		)
	}

	var elasticResp SearchResponse
	json.Unmarshal(body, &elasticResp)

	if elasticResp.Hits.Hits == nil {
		slog.Warn("Hits from elastic are null, query may be malformed")
		slog.Debug(fmt.Sprintf("Null result elastic query: %s", elasticQuery))
	}

	stripExplanation(elasticResp, query, "tool")

	return elasticResp
}

// toolsElasticConfig defines the body of the query to the elastic tools index
func toolsElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	var sortQuery []gin.H

	if query.QueryString == "" {
		if len(query.IDs) == 0 {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"match_all": gin.H{},
					},
					"random_score": gin.H{},
				},
			}
		} else {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"function_score": gin.H{
							"query": gin.H{
								"bool": gin.H{
									"filter": []gin.H{
										{
											"terms": gin.H{
												"_id": query.IDs,
											},
										},
									},
								},
							},
							"random_score": gin.H{},
						},
					},
				},
			}

			sortQuery = []gin.H{
				{
					"_script": gin.H{
						"type": "number",
						"script": gin.H{
							"lang":   "painless",
							"source": "params.order.indexOf(doc['_id'].value)",
							"params": gin.H{
								"order": query.IDs,
							},
						},
						"order": "asc",
					},
				},
			}
		}
	} else {
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
		mainQuery = gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		}
	}

	mustFilters := []gin.H{}
	for key, terms := range query.Filters["tool"] {
		filters := []gin.H{}
		for _, t := range terms.([]interface{}) {
			filters = append(filters, gin.H{"term": gin.H{key: t}})
		}
		mustFilters = append(mustFilters, gin.H{
			"bool": gin.H{
				"should": filters,
			},
		})
	}

	f1 := gin.H{
		"bool": gin.H{
			"must": mustFilters,
		},
	}

	agg1 := buildAggregations(query)

	response := gin.H{
		"size":  os.Getenv("SEARCH_NO_RECORDS"),
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"name": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
				"description": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
			},
		},
		"explain":     true,
		"post_filter": f1,
		"aggs":        agg1,
	}

	if len(query.IDs) > 0 {
		response["sort"] = sortQuery
	}

	return response
}

func CollectionSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		slog.Debug(fmt.Sprintf("Failed to interpret search query with %s", err.Error()))
		return
	}
	results := collectionSearch(query)
	BQUpload(query, results, "collection")
	c.JSON(http.StatusOK, results)
}

func collectionChannel(query Query, res chan SearchResponse) {
	elasticResp := collectionSearch(query)
	res <- elasticResp
}

// collectionsSearch performs a search of the ElasticSearch collections index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func collectionSearch(query Query) SearchResponse {
	var buf bytes.Buffer

	elasticQuery := collectionsElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to encode elastic query %s with %s",
			elasticQuery,
			err.Error()),
		)
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("collection"),
		ElasticClient.Search.WithBody(&buf),
	)

	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to execute elastic query with %s",
			err.Error()),
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to read elastic response with %s",
			err.Error()),
		)
	}

	var elasticResp SearchResponse
	json.Unmarshal(body, &elasticResp)

	if elasticResp.Hits.Hits == nil {
		slog.Warn("Hits from elastic are null, query may be malformed")
		slog.Debug(fmt.Sprintf("Null result elastic query: %s", elasticQuery))
	}

	stripExplanation(elasticResp, query, "collection")

	return elasticResp
}

// collectionsElasticConfig defines the body of the query to the elastic collections index
func collectionsElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	var sortQuery []gin.H

	if query.QueryString == "" {
		if len(query.IDs) == 0 {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"match_all": gin.H{},
					},
					"random_score": gin.H{},
				},
			}
		} else {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"function_score": gin.H{
							"query": gin.H{
								"bool": gin.H{
									"filter": []gin.H{
										{
											"terms": gin.H{
												"_id": query.IDs,
											},
										},
									},
								},
							},
							"random_score": gin.H{},
						},
					},
				},
			}

			sortQuery = []gin.H{
				{
					"_script": gin.H{
						"type": "number",
						"script": gin.H{
							"lang":   "painless",
							"source": "params.order.indexOf(doc['_id'].value)",
							"params": gin.H{
								"order": query.IDs,
							},
						},
						"order": "asc",
					},
				},
			}
		}
	} else {
		relatedObjectFields := []string{
			"datasetTitles",
			"datasetAbstracts",
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
		mainQuery = gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		}
	}

	mustFilters := []gin.H{}
	for key, terms := range query.Filters["collection"] {
		filters := []gin.H{}
		for _, t := range terms.([]interface{}) {
			filters = append(filters, gin.H{"term": gin.H{key: t}})
		}
		mustFilters = append(mustFilters, gin.H{
			"bool": gin.H{
				"should": filters,
			},
		})
	}

	f1 := gin.H{
		"bool": gin.H{
			"must": mustFilters,
		},
	}

	agg1 := buildAggregations(query)

	response := gin.H{
		"size":  os.Getenv("SEARCH_NO_RECORDS"),
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"description": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
				"name": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
				"keywords": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
			},
		},
		"explain":     true,
		"post_filter": f1,
		"aggs":        agg1,
	}

	if len(query.IDs) > 0 {
		response["sort"] = sortQuery
	}

	return response
}

func DataUseSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		slog.Debug(fmt.Sprintf("Failed to interpret search query with %s", err.Error()))
		return
	}
	results := dataUseSearch(query)
	BQUpload(query, results, "datauseregister")
	c.JSON(http.StatusOK, results)
}

func dataUseChannel(query Query, res chan SearchResponse) {
	elasticResp := dataUseSearch(query)
	res <- elasticResp
}

// dataUseSearch performs a search of the ElasticSearch data uses index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func dataUseSearch(query Query) SearchResponse {
	var buf bytes.Buffer

	elasticQuery := dataUseElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to encode elastic query %s with %s",
			elasticQuery,
			err.Error()),
		)
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("datauseregister"),
		ElasticClient.Search.WithBody(&buf),
	)

	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to execute elastic query with %s",
			err.Error()),
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to read elastic response with %s",
			err.Error()),
		)
	}

	var elasticResp SearchResponse
	json.Unmarshal(body, &elasticResp)

	if elasticResp.Hits.Hits == nil {
		slog.Warn("Hits from elastic are null, query may be malformed")
		slog.Debug(fmt.Sprintf("Null result elastic query: %s", elasticQuery))
	}

	stripExplanation(elasticResp, query, "dur")

	return elasticResp
}

// dataUseElasticConfig defines the body of the query to the elastic data uses index
func dataUseElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	var sortQuery []gin.H

	if query.QueryString == "" {
		if len(query.IDs) == 0 {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"match_all": gin.H{},
					},
					"random_score": gin.H{},
				},
			}
		} else {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"function_score": gin.H{
							"query": gin.H{
								"bool": gin.H{
									"filter": []gin.H{
										{
											"terms": gin.H{
												"_id": query.IDs,
											},
										},
									},
								},
							},
							"random_score": gin.H{},
						},
					},
				},
			}

			sortQuery = []gin.H{
				{
					"_script": gin.H{
						"type": "number",
						"script": gin.H{
							"lang":   "painless",
							"source": "params.order.indexOf(doc['_id'].value)",
							"params": gin.H{
								"order": query.IDs,
							},
						},
						"order": "asc",
					},
				},
			}
		}
	} else {
		searchableFields := []string{
			"projectTitle",
			"laySummary",
			"publicBenefitStatement",
			"technicalSummary",
			"fundersAndSponsors",
			"datasetTitles",
			"keywords",
			"collectionNames",
			"publisherName",
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
		mainQuery = gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		}
	}

	mustFilters := []gin.H{}
	for key, terms := range query.Filters["dataUseRegister"] {
		filters := []gin.H{}
		for _, t := range terms.([]interface{}) {
			filters = append(filters, gin.H{"term": gin.H{key: t}})
		}
		mustFilters = append(mustFilters, gin.H{
			"bool": gin.H{
				"should": filters,
			},
		})
	}

	f1 := gin.H{
		"bool": gin.H{
			"must": mustFilters,
		},
	}

	agg1 := buildAggregations(query)

	response := gin.H{
		"size":  os.Getenv("SEARCH_NO_RECORDS"),
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"laySummary": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
			},
		},
		"explain":     true,
		"post_filter": f1,
		"aggs":        agg1,
	}

	if len(query.IDs) > 0 {
		response["sort"] = sortQuery
	}

	return response
}

func PublicationSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		slog.Debug(fmt.Sprintf("Failed to interpret search query with %s", err.Error()))
		return
	}
	results := publicationSearch(query)
	BQUpload(query, results, "publication")
	c.JSON(http.StatusOK, results)
}

func publicationChannel(query Query, res chan SearchResponse) {
	elasticResp := publicationSearch(query)
	res <- elasticResp
}

// publicationSearch performs a search of the ElasticSearch publications index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
// The publications index consists of the publications that are hosted on the
// Gateway - this is not a federated search.
func publicationSearch(query Query) SearchResponse {
	var buf bytes.Buffer

	elasticQuery := publicationElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to encode elastic query %s with %s",
			elasticQuery,
			err.Error()),
		)
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("publication"),
		ElasticClient.Search.WithBody(&buf),
	)

	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to execute elastic query with %s",
			err.Error()),
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to read elastic response with %s",
			err.Error()),
		)
	}

	var elasticResp SearchResponse
	json.Unmarshal(body, &elasticResp)

	if elasticResp.Hits.Hits == nil {
		slog.Warn("Hits from elastic are null, query may be malformed")
		slog.Debug(fmt.Sprintf("Null result elastic query: %s", elasticQuery))
	}

	stripExplanation(elasticResp, query, "publication")

	return elasticResp
}

// publicationElasticConfig defines the body of the query to the elastic publications index
func publicationElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	var sortQuery []gin.H

	if query.QueryString == "" {
		if len(query.IDs) == 0 {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"match_all": gin.H{},
					},
					"random_score": gin.H{},
				},
			}
		} else {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"function_score": gin.H{
							"query": gin.H{
								"bool": gin.H{
									"filter": []gin.H{
										{
											"terms": gin.H{
												"_id": query.IDs,
											},
										},
									},
								},
							},
							"random_score": gin.H{},
						},
					},
				},
			}

			sortQuery = []gin.H{
				{
					"_script": gin.H{
						"type": "number",
						"script": gin.H{
							"lang":   "painless",
							"source": "params.order.indexOf(doc['_id'].value)",
							"params": gin.H{
								"order": query.IDs,
							},
						},
						"order": "asc",
					},
				},
			}
		}
	} else {
		searchableFields := []string{
			"title",
			"journalName",
			"abstract",
			"publicationType",
			"authors",
			"datasetTitles",
			"doi",
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
		mainQuery = gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		}
	}

	mustFilters := []gin.H{}
	for key, terms := range query.Filters["paper"] {
		filters := []gin.H{}
		if key == "publicationDate" {
			rangeFilter := gin.H{
				"bool": gin.H{
					"must": []gin.H{
						{"range": gin.H{"publicationDate": gin.H{"gte": terms.([]interface{})[0]}}},
						{"range": gin.H{"publicationDate": gin.H{"lte": terms.([]interface{})[1]}}},
					},
				},
			}
			mustFilters = append(mustFilters, rangeFilter)
		} else {
			for _, t := range terms.([]interface{}) {
				filters = append(filters, gin.H{"term": gin.H{key: t}})
			}
			mustFilters = append(mustFilters, gin.H{
				"bool": gin.H{
					"should": filters,
				},
			})
		}
	}

	f1 := gin.H{
		"bool": gin.H{
			"must": mustFilters,
		},
	}

	agg1 := buildAggregations(query)

	response := gin.H{
		"size":  os.Getenv("SEARCH_NO_RECORDS"),
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"title": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
				"abstract": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
			},
		},
		"explain":     true,
		"post_filter": f1,
		"aggs":        agg1,
	}

	if len(query.IDs) > 0 {
		response["sort"] = sortQuery
	}

	return response
}

func DataProviderSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		slog.Debug(fmt.Sprintf("Failed to interpret search query with %s", err.Error()))
		return
	}

	results := dataProviderSearch(query)
	BQUpload(query, results, "dataprovider")
	c.JSON(http.StatusOK, results)
}

func dataProviderChannel(query Query, res chan SearchResponse) {
	elasticResp := dataProviderSearch(query)
	res <- elasticResp
}

// dataProviderSearch performs a search of the ElasticSearch dataproviders index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func dataProviderSearch(query Query) SearchResponse {
	var buf bytes.Buffer

	elasticQuery := dataProviderElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to encode elastic query %s with %s",
			elasticQuery,
			err.Error()),
		)
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("dataprovider"),
		ElasticClient.Search.WithBody(&buf),
	)

	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to execute elastic query with %s",
			err.Error()),
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to read elastic response with %s",
			err.Error()),
		)
	}

	var elasticResp SearchResponse
	json.Unmarshal(body, &elasticResp)

	if elasticResp.Hits.Hits == nil {
		slog.Warn("Hits from elastic are null, query may be malformed")
		slog.Debug(fmt.Sprintf("Null result elastic query: %s", elasticQuery))
	}

	stripExplanation(elasticResp, query, "dataProvider")

	return elasticResp
}

// dataProviderElasticConfig defines the body of the query to the elastic data providers index
func dataProviderElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	var sortQuery []gin.H

	if query.QueryString == "" {
		if len(query.IDs) == 0 {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"match_all": gin.H{},
					},
					"random_score": gin.H{},
				},
			}
		} else {
			mainQuery = gin.H{
				"function_score": gin.H{
					"query": gin.H{
						"function_score": gin.H{
							"query": gin.H{
								"bool": gin.H{
									"filter": []gin.H{
										{
											"terms": gin.H{
												"_id": query.IDs,
											},
										},
									},
								},
							},
							"random_score": gin.H{},
						},
					},
				},
			}

			sortQuery = []gin.H{
				{
					"_script": gin.H{
						"type": "number",
						"script": gin.H{
							"lang":   "painless",
							"source": "params.order.indexOf(doc['_id'].value)",
							"params": gin.H{
								"order": query.IDs,
							},
						},
						"order": "asc",
					},
				},
			}
		}
	} else {
		searchableFields := []string{
			"name",
			"datasetTitles",
			"geographicLocation",
			"publicationTitles",
			"collectionNames",
			"durTitles",
			"toolNames",
			"teamAliases",
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
		mainQuery = gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		}
	}

	mustFilters := []gin.H{}
	for key, terms := range query.Filters["dataProvider"] {
		filters := []gin.H{}
		for _, t := range terms.([]interface{}) {
			filters = append(filters, gin.H{"term": gin.H{key: t}})
		}
		mustFilters = append(mustFilters, gin.H{
			"bool": gin.H{
				"should": filters,
			},
		})
	}

	f1 := gin.H{
		"bool": gin.H{
			"must": mustFilters,
		},
	}

	agg1 := buildAggregations(query)

	response := gin.H{
		"size":        os.Getenv("SEARCH_NO_RECORDS"),
		"query":       mainQuery,
		"explain":     true,
		"post_filter": f1,
		"aggs":        agg1,
	}

	if len(query.IDs) > 0 {
		response["sort"] = sortQuery
	}

	return response
}

// DataCustodianNetworkSearch performs a search of the ElasticSearch dataCustodianNetworks index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func DataCustodianNetworkSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		slog.Debug(fmt.Sprintf("Failed to interpret search query with %s", err.Error()))
		return
	}
	results := dataCustodianNetworkSearch(query)
	BQUpload(query, results, "datacustodiannetwork")
	c.JSON(http.StatusOK, results)
}

func dataCustodianNetworkChannel(query Query, res chan SearchResponse) {
	elasticResp := dataCustodianNetworkSearch(query)
	res <- elasticResp
}

// dataCustodianNetworkSearch performs a search of the ElasticSearch dataCustodianNetworks index using
// the provided query as the search term.  Results are returned in the format
// returned by elastic (SearchResponse).
func dataCustodianNetworkSearch(query Query) SearchResponse {
	var buf bytes.Buffer

	elasticQuery := dataCustodianNetworkElasticConfig(query)
	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to encode elastic query %s with %s",
			elasticQuery,
			err.Error()),
		)
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("datacustodiannetwork"),
		ElasticClient.Search.WithBody(&buf),
	)

	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to execute elastic query with %s",
			err.Error()),
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to read elastic response with %s",
			err.Error()),
		)
	}

	var elasticResp SearchResponse
	json.Unmarshal(body, &elasticResp)

	if elasticResp.Hits.Hits == nil {
		slog.Warn("Hits from elastic are null, query may be malformed")
		slog.Debug(fmt.Sprintf("Null result elastic query: %s", elasticQuery))
	}

	stripExplanation(elasticResp, query, "datacustodiannetwork")

	return elasticResp
}

// dataCustodianNetworkElasticConfig defines the body of the query to the elastic datacustodiannetwork index
func dataCustodianNetworkElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	if query.QueryString == "" {
		mainQuery = gin.H{
			"function_score": gin.H{
				"query": gin.H{
					"match_all": gin.H{},
				},
				"random_score": gin.H{},
			},
		}
	} else {
		relatedObjectFields := []string{
			"publisherNames",
			"datasetTitles",
			"durTitles",
			"toolNames",
			"publicationTitles",
			"collectionNames",
		}
		searchableFields := []string{
			"name",
			"summary",
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
		mainQuery = gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		}
	}

	mustFilters := []gin.H{}
	for key, terms := range query.Filters["datacustodiannetwork"] {
		filters := []gin.H{}
		for _, t := range terms.([]interface{}) {
			filters = append(filters, gin.H{"term": gin.H{key: t}})
		}
		mustFilters = append(mustFilters, gin.H{
			"bool": gin.H{
				"should": filters,
			},
		})
	}

	f1 := gin.H{
		"bool": gin.H{
			"must": mustFilters,
		},
	}

	agg1 := buildAggregations(query)

	return gin.H{
		"size":  os.Getenv("SEARCH_NO_RECORDS"),
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"name": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
				"summary": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size":    0,
					"no_match_size":    0,
				},
			},
		},
		"explain":     true,
		"post_filter": f1,
		"aggs":        agg1,
	}
}

// buildAggregations constructs the "aggs" part of an elastic search query
// from provided Aggregations.
// Aggregations are expected to be an array of `{'type': string, 'keys': string}`
func buildAggregations(query Query) gin.H {
	agg1 := gin.H{}
	for _, agg := range query.Aggregations {
		k, ok := agg["keys"].(string)
		if !ok {
			log.Printf("Filter key in %s not recognised", agg)
		}
		if k == "dateRange" {
			agg1["startDate"] = gin.H{"min": gin.H{"field": "startDate"}}
			agg1["endDate"] = gin.H{"max": gin.H{"field": "endDate"}}
		} else if k == "publicationDate" {
			agg1["startDate"] = gin.H{"min": gin.H{"field": "publicationDate"}}
			agg1["endDate"] = gin.H{"max": gin.H{"field": "publicationDate"}}
		} else if k == "populationSize" {
			ranges := populationRanges()
			agg1[k] = gin.H{
				"range": gin.H{"field": k, "ranges": ranges},
			}
		} else {
			agg1[k] = gin.H{"terms": gin.H{"field": k, "size": os.Getenv("SEARCH_NO_RECORDS_AGGREGATION")}}
		}
	}
	return agg1
}
func populationRanges() []gin.H {
	var ranges []gin.H
	ranges = append(ranges, gin.H{"from": -1.0, "to": 1.0, "key": "Unreported"})
	for i := 0; i < 9; i++ {
		ranges = append(ranges, gin.H{"from": math.Pow(10, float64(i)), "to": math.Pow(10, float64(i+1))})
	}
	return ranges
}

// Remove the explanations from a SearchResponse to reduce its size
// And send explanation to search explanation extractor
func stripExplanation(elasticResp SearchResponse, query Query, entityType string) {
	_, expEnabled := os.LookupEnv("SEARCH_EXPLANATION_EXTRACTOR")
	// Send explanation if enabled, entity is dataset and query is not empty
	if expEnabled && entityType == "dataset" && !reflect.ValueOf(query).IsZero() {
		respCopy := copyResponseHits(elasticResp)
		go extractExplanation(respCopy, query)
	}

	for i := range elasticResp.Hits.Hits {
		elasticResp.Hits.Hits[i].Explanation = make(map[string]interface{}, 0)
	}
}

func copyResponseHits(r SearchResponse) SearchResponse {
	var hits []Hit
	hits = append(hits, r.Hits.Hits...)
	hitsField := HitsField{
		Hits: hits,
	}
	return SearchResponse{
		Hits: hitsField,
	}
}

func extractExplanation(elasticResp SearchResponse, query Query) {
	bodyContent := gin.H{
		"data":              elasticResp,
		"query":             fmt.Sprintf("%s", query),
		"destination_table": os.Getenv("SEARCH_EXPLANATION_TABLE"),
	}
	body, err := json.Marshal(bodyContent)
	if err != nil {
		slog.Info(fmt.Sprintf("Failed to marshal search explanation payload: %s", err.Error()))
	}

	urlPath := fmt.Sprintf("%s/process_data", os.Getenv("SEARCH_EXPLANATION_EXTRACTOR"))
	req, err := http.NewRequest("POST", urlPath, bytes.NewBuffer(body))
	if err != nil {
		slog.Info(fmt.Sprintf("Failed to build search explanation payload with: %s", err.Error()))
	}
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(os.Getenv("SEARCH_EXPLANATION_USER"), os.Getenv("SEARCH_EXPLANATION_PASSWORD"))

	response, err := Client.Do(req)
	if err != nil {
		slog.Info(fmt.Sprintf("Failed to execute query with: %s", err.Error()))
	}
	defer response.Body.Close()

	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Info(fmt.Sprintf(
			"Failed to extract search explanation with error %s", err.Error(),
		))
	}
	slog.Debug(fmt.Sprintf(
		"Search explanation extraction routine exited with response: %s", respBody,
	))
}

// SearchSimilarDatasets returns the top 3 datasets similar to the document with
// the provided id.
func SearchSimilarDatasets(c *gin.Context) {
	var querySimilar SimilarSearch
	if err := c.BindJSON(&querySimilar); err != nil {
		slog.Debug(fmt.Sprintf("Failed to interpret search query with %s", err.Error()))
		return
	}

	results := similarSearch(querySimilar.ID, "dataset")
	c.JSON(http.StatusOK, results)
}

func similarSearch(id string, index string) SearchResponse {
	var buf bytes.Buffer

	elasticQuery := gin.H{
		"size": os.Getenv("SEARCH_NO_RECORDS_SIMILAR_SEARCH"),
		"query": gin.H{
			"more_like_this": gin.H{
				"like": []gin.H{
					{"_index": index, "_id": id},
				},
			},
		},
	}

	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to encode elastic query %s with %s",
			elasticQuery,
			err.Error()),
		)
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex(index),
		ElasticClient.Search.WithBody(&buf),
	)

	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to execute elastic query with %s",
			err.Error()),
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Debug(fmt.Sprintf(
			"Failed to read elastic response with %s",
			err.Error()),
		)
	}

	var elasticResp SearchResponse
	json.Unmarshal(body, &elasticResp)

	if elasticResp.Hits.Hits == nil {
		slog.Warn("Hits from elastic are null, query may be malformed")
		slog.Debug(fmt.Sprintf("Null result elastic query: %s", elasticQuery))
	}

	return elasticResp
}

func uploadSearchAnalytics(query Query, results SearchResponse, entityType string) {

	ctx := context.Background()
	analyticsDataset := BigQueryClient.Dataset(os.Getenv("BQ_DATASET_NAME"))
	table := analyticsDataset.Table(os.Getenv("BQ_TABLE_NAME"))

	schema := bigquery.Schema{
		{Name: "Timestamp", Required: false, Type: bigquery.DateTimeFieldType},
		{Name: "EntityType", Required: true, Type: bigquery.StringFieldType},
		{Name: "SearchTerm", Required: false, Type: bigquery.StringFieldType},
		{Name: "FilterUsed", Repeated: false, Type: bigquery.JSONFieldType},
		{Name: "PageResults", Required: false, Type: bigquery.JSONFieldType},
		{Name: "EntitiesReturned", Required: true, Type: bigquery.IntegerFieldType},
	}

	if err := table.Create(ctx, &bigquery.TableMetadata{Schema: schema}); err != nil {
		var e *googleapi.Error
		if errors.As(err, &e) {
			if e.Code == 409 {
				slog.Debug(fmt.Sprintf("%s", err.Error()))
			}
		} else {
			slog.Info(fmt.Sprintf("Could not create table: %s", err.Error()))
		}
	}

	u := table.Inserter()

	var datasetIds []string
	for _, r := range results.Hits.Hits {
		datasetIds = append(datasetIds, r.Id)
	}
	pageResults, err := json.Marshal(gin.H{"entity_ids": datasetIds})
	if err != nil {
		slog.Info(fmt.Sprintf("Could not marshal page results: %s", err.Error()))
	}

	filterUsed, err := json.Marshal(query.Filters)
	if err != nil {
		slog.Info(fmt.Sprintf("Could not marshal filters: %s", err.Error()))
	}

	searchResult := SearchAnalytics{
		Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
		EntityType:       entityType,
		SearchTerm:       query.QueryString,
		FilterUsed:       string(filterUsed),
		PageResults:      string(pageResults),
		EntitiesReturned: int(results.Hits.Total["value"].(float64)),
	}

	if err := u.Put(ctx, searchResult); err != nil {
		slog.Info(fmt.Sprintf("Failed to upload search analytics to BigQuery: %s", err.Error()))
	}
}
