package search

import (
	"encoding/json"
	"hdruk/search-service/utils/mocks"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)


func init() {
	ElasticClient = mocks.MockElasticClient()
}

func TestListDatasetFilters(t *testing.T) {
	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockGet(c)

	ListDatasetFilters(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp map[string]interface{}
	json.Unmarshal(bodyBytes, &testResp)

	assert.Contains(t, testResp, "took")
	assert.Contains(t, testResp, "aggregations")
}