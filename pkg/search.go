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

/* Query represents the search query incoming from the gateway-api
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
	QueryString string `json:"query"`
	Filters map[string]map[string][]interface{} `json:"filters"`
	Aggregations	[]map[string]interface{}	`json:"aggs"`
}

type SimilarSearch struct {
	ID	string	`json:"id"`
}

// SearchResponse represents the expected structure of results returned by ElasticSearch
type SearchResponse struct {
	Took     int                    `json:"took"`
	TimedOut bool                   `json:"timed_out"`
	Shards   map[string]interface{} `json:"_shards"`
	Hits	HitsField `json:"hits"`
	Aggregations map[string]interface{}	`json:"aggregations"`
}

type HitsField struct {
	Total	map[string]interface{}	`json:"total"`
	MaxScore	float64	`json:"max_score"`
	Hits	[]Hit	`json:"hits"`
}

type Hit struct {
	Explanation	map[string]interface{}	`json:"_explanation"`
	Id	string	`json:"_id"`
    Score	float64	`json:"_score"`   
    Source	map[string]interface{}	`json:"_source"`
	Highlight	map[string][]string	`json:"highlight"`
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

	results := make(map[string]interface{})

	go datasetChannel(query, datasetResults)
	go toolChannel(query, toolResults)
	go collectionChannel(query, collectionResults)
	go dataUseChannel(query, dataUseResults)

	for i := 0; i < 4; i++ {
		select {
		case datasets := <-datasetResults:
			results["dataset"] = datasets
		case tools := <-toolResults:
			results["tool"] = tools
		case collections := <-collectionResults:
			results["collection"] = collections
		case data_uses := <-dataUseResults:
			results["dataUseRegister"] = data_uses
		}
	}

	c.JSON(http.StatusOK, results)
}

func DatasetSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		return
	}

	results := datasetSearch(query)
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
		log.Fatal(err.Error())
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("dataset"),
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

	stripExplanation(elasticResp)

	return elasticResp
}

// datasetElasticConfig defines the body of the query to the elastic datasets index
func datasetElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	if query.QueryString == "" {
		mainQuery = gin.H{
			"match_all": gin.H{},
		}
	} else {
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
		mainQuery = gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		}
	}

	mustFilters := []gin.H{}
	for key, terms := range(query.Filters["dataset"]) {
		filters := []gin.H{}
		if (key == "dateRange") {
			rangeFilter := gin.H{
				"bool": gin.H{
                    "must": []gin.H{
                        {"range": gin.H{"startDate": gin.H{"lte": terms[1]}}},
                        {"range": gin.H{"endDate": gin.H{"gte": terms[0]}}},
					},
                },
			}
			mustFilters = append(mustFilters, rangeFilter)
		} else {
			for _, t := range(terms) {
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

	return gin.H{
		"size": 100,
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"description": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size": 0,
					"no_match_size": 0,
				},
				"abstract": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size": 0,
					"no_match_size": 0,
				},
			},
		},
		"explain": true,
		"post_filter": f1,
		"aggs": agg1,
	}
}

func ToolSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		return
	}
	results := toolSearch(query)
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
		log.Fatal(err.Error())
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("tool"),
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

	stripExplanation(elasticResp)

	return elasticResp
}

// toolsElasticConfig defines the body of the query to the elastic tools index
func toolsElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	if query.QueryString == "" {
		mainQuery = gin.H{
			"match_all": gin.H{},
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
	for key, terms := range(query.Filters["tool"]) {
		filters := []gin.H{}
		for _, t := range(terms) {
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
		"size": 100,
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"name":        gin.H{
					"boundary_scanner": "sentence",
					"fragment_size": 0,
					"no_match_size": 0,
				},
				"description": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size": 0,
					"no_match_size": 0,
				},
			},
		},
		"explain": true,
		"post_filter": f1,
		"aggs": agg1,
	}
}

func CollectionSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		return
	}
	results := collectionSearch(query)
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
		log.Fatal(err.Error())
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("collection"),
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

	stripExplanation(elasticResp)

	return elasticResp
}

// collectionsElasticConfig defines the body of the query to the elastic collections index
func collectionsElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	if query.QueryString == "" {
		mainQuery = gin.H{
			"match_all": gin.H{},
		}
	} else {
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
		mainQuery = gin.H{
			"bool": gin.H{
				"should": []gin.H{mm1, mm2, mm3},
			},
		}
	}

	mustFilters := []gin.H{}
	for key, terms := range(query.Filters["collection"]) {
		filters := []gin.H{}
		for _, t := range(terms) {
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
		"size": 100,
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"description": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size": 0,
					"no_match_size": 0,
				},
				"name":        gin.H{
					"boundary_scanner": "sentence",
					"fragment_size": 0,
					"no_match_size": 0,
				},
				"keywords":    gin.H{
					"boundary_scanner": "sentence",
					"fragment_size": 0,
					"no_match_size": 0,
				},
			},
		},
		"explain": true,
		"post_filter": f1,
		"aggs": agg1,
	}
}

func DataUseSearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		return
	}
	results := dataUseSearch(query)
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
		log.Fatal(err.Error())
	}

	response, err := ElasticClient.Search(
		ElasticClient.Search.WithIndex("datauseregister"),
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

	stripExplanation(elasticResp)

	return elasticResp
}

// dataUseElasticConfig defines the body of the query to the elastic data uses index
func dataUseElasticConfig(query Query) gin.H {
	var mainQuery gin.H
	if query.QueryString == "" {
		mainQuery = gin.H{
			"match_all": gin.H{},
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
	for key, terms := range(query.Filters["datauseregister"]) {
		filters := []gin.H{}
		for _, t := range(terms) {
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
		"size": 100,
		"query": mainQuery,
		"highlight": gin.H{
			"fields": gin.H{
				"laySummary": gin.H{
					"boundary_scanner": "sentence",
					"fragment_size": 0,
					"no_match_size": 0,
				},
			},
		},
		"explain": true,
		"post_filter": f1,
		"aggs": agg1,
	}
}

// buildAggregations constructs the "aggs" part of an elastic search query
// from provided Aggregations.
// Aggregations are expected to be an array of `{'type': string, 'keys': string}`
func buildAggregations(query Query) gin.H {
	agg1 := gin.H{}
	for _, agg := range(query.Aggregations) {
		k, ok := agg["keys"].(string)
		if !ok {
			log.Printf("Filter key in %s not recognised", agg)
		}
		if (k == "dateRange") {
			agg1["startDate"] = gin.H{"min": gin.H{"field": "startDate"}}
			agg1["endDate"] = gin.H{"max": gin.H{"field": "endDate"}}
		} else {
			agg1[k] = gin.H{"terms": gin.H{"field": k, "size": 1000}}
		}
	}
	return agg1
}

// Remove the explanations from a SearchResponse to reduce its size
func stripExplanation(elasticResp SearchResponse) {
	var explanations []map[string]interface{}

	for i, hit := range elasticResp.Hits.Hits {
		explanations = append(explanations, hit.Explanation)
		elasticResp.Hits.Hits[i].Explanation = make(map[string]interface{}, 0)
	}

	// TO DO - send explanations to BigQuery
}

// SearchSimilarDatasets returns the top 3 datasets similar to the document with
// the provided id.
func SearchSimilarDatasets(c *gin.Context) {
	var querySimilar SimilarSearch
	if err := c.BindJSON(&querySimilar); err != nil {
		return
	}

	results := similarSearch(querySimilar.ID, "dataset")
	c.JSON(http.StatusOK, results)
}

func similarSearch(id string, index string) SearchResponse {
	var buf bytes.Buffer

	elasticQuery := gin.H{
		"size": 3,
		"query": gin.H{
			"more_like_this": gin.H{
				"like": []gin.H{
					{"_index": index, "_id": id},
				},
			},
		},
	}

	if err := json.NewEncoder(&buf).Encode(elasticQuery); err != nil {
		log.Fatal(err.Error())
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

	return elasticResp
}