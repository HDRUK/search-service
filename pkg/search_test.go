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