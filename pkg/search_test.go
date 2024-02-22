package search

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"hdruk/search-service/utils/mocks"
)

func init() {
	ElasticClient = mocks.MockElasticClient()
}

func GetTestGinContext(w *httptest.ResponseRecorder) *gin.Context {
	gin.SetMode(gin.TestMode)

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = &http.Request{
		Header: make(http.Header),
	}

	return ctx
}

func MockPostToSearch(c *gin.Context) {
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	bodyContent := gin.H{"query": "test query"}
	bodyBytes, err := json.Marshal(bodyContent)
	if err != nil {
		log.Fatal(err.Error())
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}

func MockPostToSimilarSearch(c *gin.Context) {
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	bodyContent := gin.H{"id": "1"}
	bodyBytes, err := json.Marshal(bodyContent)
	if err != nil {
		log.Fatal(err.Error())
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}

func TestSearchGeneric(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostToSearch(c)

	SearchGeneric(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.Contains(t, testResp, "dataset")
	assert.Contains(t, testResp, "tool")
	assert.Contains(t, testResp, "collection")
	assert.Contains(t, testResp, "dataUseRegister")

	datasetResp := testResp["dataset"].(map[string]interface{})
	assert.EqualValues(t, 3, int(datasetResp["took"].(float64)))
}

func TestDatasetSearch(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostToSearch(c)

	DatasetSearch(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.Contains(t, testResp, "hits")
	assert.Contains(t, testResp, "took")
	assert.EqualValues(t, 3, int(testResp["took"].(float64)))
}

func TestToolSearch(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostToSearch(c)

	ToolSearch(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.Contains(t, testResp, "hits")
	assert.Contains(t, testResp, "took")
	assert.EqualValues(t, 3, int(testResp["took"].(float64)))
}

func TestCollectionSearch(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostToSearch(c)

	CollectionSearch(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.Contains(t, testResp, "hits")
	assert.Contains(t, testResp, "took")
	assert.EqualValues(t, 3, int(testResp["took"].(float64)))
}

func TestDataUseSearch(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostToSearch(c)

	DataUseSearch(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.Contains(t, testResp, "hits")
	assert.Contains(t, testResp, "took")
	assert.EqualValues(t, 3, int(testResp["took"].(float64)))
}

func TestSimilarDatasetSearch(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostToSimilarSearch(c)

	SearchSimilarDatasets(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.Contains(t, testResp, "hits")
	assert.Contains(t, testResp, "took")
	assert.EqualValues(t, 3, int(testResp["took"].(float64)))
}

func TestDatasetElasticConfig(t *testing.T) {
	TestQuery := Query{
		QueryString: "search term test",
		Filters: map[string]map[string][]interface{}{
			"dataset": {
				"publisherName": []interface{}{
					"publisher A",
					"publisher B",
				},
				"dataType": []interface{}{
					"data type A",
				},
				"dateRange": []interface{}{
					"2020",
					"2021",
				},
			},
		},
		Aggregations: []map[string]interface{}{
			{
				"type": "dataset",
				"keys": "publisherName",
			},
			{
				"type": "dataset",
				"keys": "dataType",
			},
		},
	}

	datasetConfig := datasetElasticConfig(TestQuery)

	// assert query clause exists and that it contains query term
	assert.Contains(t, datasetConfig, "query")
	queryJson, _ := json.Marshal(datasetConfig)
	queryStr := string(queryJson)
	assert.Contains(t, queryStr, "search term test")

	// assert filter clause exists and contains boolean query
	assert.Contains(t, datasetConfig, "post_filter")
	filterClause := datasetConfig["post_filter"].(gin.H)
	assert.Contains(t, filterClause, "bool")
	assert.Contains(t, filterClause["bool"], "must")

	// assert specific filter keys are included
	assert.Contains(t, queryStr, "\"publisherName\":\"publisher A\"")
	assert.Contains(t, queryStr, "\"publisherName\":\"publisher B\"")
	assert.Contains(t, queryStr, "\"dataType\":\"data type A\"")
	assert.Contains(t, queryStr, "\"lte\":\"2021\"")
	assert.Contains(t, queryStr, "\"gte\":\"2020\"")
	
	// assert aggregations clause exists and contains specific keys
	assert.Contains(t, datasetConfig, "aggs")
	aggsClause := datasetConfig["aggs"].(gin.H)
	assert.Contains(t, aggsClause, "publisherName")
	assert.Contains(t, aggsClause, "dataType")
}