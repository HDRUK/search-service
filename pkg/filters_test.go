package search

import (
	"bytes"
	"encoding/json"
	"hdruk/search-service/utils/mocks"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)


func init() {
	ElasticClient = mocks.MockElasticClient()
}

func MockPostFilters(c *gin.Context) {
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	bodyContent := gin.H{
		"filters": []gin.H{
			{
				"type": "dataset",
				"keys": "publisherName",
			},
		},
	}
	bodyBytes, err := json.Marshal(bodyContent)
	if err != nil {
		log.Fatal(err.Error())
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}

func TestListFilters(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostFilters(c)

	ListFilters(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.Contains(t, testResp, "filters")
	assert.Contains(t, testResp["filters"].([]interface{})[0], "dataset")
}