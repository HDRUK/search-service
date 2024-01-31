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

func MockGet(c *gin.Context) {
	c.Request.Method = "GET"
	c.Request.Header.Set("Content-Type", "application/json")
	bodyContent := gin.H{"query": "test query"}
	bodyBytes, err := json.Marshal(bodyContent)
	if err != nil {
		log.Fatal(err.Error())
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}

func TestSearchGeneric(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockGet(c)

	SearchGeneric(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.Contains(t, testResp, "datasets")
	assert.Contains(t, testResp, "tools")
	assert.Contains(t, testResp, "collections")
	assert.Contains(t, testResp, "data_uses")

	datasetResp := testResp["datasets"].(map[string]interface{})
	assert.EqualValues(t, 3, int(datasetResp["took"].(float64)))
}

func TestDatasetSearch(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockGet(c)

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
	MockGet(c)

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
	MockGet(c)

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
	MockGet(c)

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