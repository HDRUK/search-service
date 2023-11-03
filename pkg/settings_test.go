package search

import (
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

func MockPost(c *gin.Context) {
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
}

func TestDefineDatasetSettings(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPost(c)

	DefineDatasetSettings(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.EqualValues(t, gin.H{"acknowledged": true}, testResp)
}

func TestDefineToolSettings(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPost(c)

	DefineToolSettings(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.EqualValues(t, gin.H{"acknowledged": true}, testResp)
}