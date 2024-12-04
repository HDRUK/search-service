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

var epmcRespJson = `{
	"version": "10.1",
	"hitCount": 1,
	"request": {
		"queryString": "DOI:10.123/abc",
		"resultType": "core",
		"cursorMark": "*",
		"pageSize": 25,
		"sort": "",
		"synonym": false
	},
	"resultList": {
		"result": [
			{
				"id": "0000000",
				"source": "MED",
				"pmid": "000000",
				"pmcid": "PMC000000",
				"fullTextIdList": {
					"fullTextId": [
						"PMC000000"
					]
				},
				"doi": "10.123/abc",
				"title": "A publication",
				"authorString": "Monday A, Tuesday B, Wednesday C",
				"journalInfo": {
					"journal": {
						"title": "Journal of Health"
					}
				},
				"pubYear": "2020",
				"abstractText": "A longer description of the paper",
				"pubTypeList": {
					"pubType": [
						"research-article",
						"Journal Article"
					]
				}
			}
		]
	}
}`

func init() {
	Client = &mocks.MockClient{}
}

func MockPostToDOISearch(c *gin.Context) {
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	bodyContent := gin.H{"query": "https://doi.org/10.1010/a11-22(22)33v3"}
	bodyBytes, err := json.Marshal(bodyContent)
	if err != nil {
		log.Fatal(err.Error())
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}

func MockPostToFieldSearch(c *gin.Context) {
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	bodyContent := gin.H{
		"query": "A Very Useful Dataset (AVUD)",
		"field": []string{
			"TITLE","ABSTRACT","METHODS",
		},
		"filters": gin.H{
			"paper": gin.H{
				"publicationDate": []string{"2020", "2021"},
				"publicationType": []string{"Review articles", "Preprints"},
			},
		},
	}
	bodyBytes, err := json.Marshal(bodyContent)
	if err != nil {
		log.Fatal(err.Error())
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}

func MockPostToArrayFieldSearch(c *gin.Context) {
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	bodyContent := gin.H{
		"query": []string{"A Very Useful Dataset (AVUD)", "A Second Very Useful Dataset (ASVUD)"},
		"field": []string{
			"TITLE","ABSTRACT","METHODS",
		},
		"filters": gin.H{
			"paper": gin.H{
				"publicationDate": []string{"2020", "2021"},
				"publicationType": []string{"Review articles", "Preprints"},
			},
		},
	}
	bodyBytes, err := json.Marshal(bodyContent)
	if err != nil {
		log.Fatal(err.Error())
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}

func TestExtractDOI(t *testing.T) {
	doi := "https://doi.org/10.1010/a11-22(22)33v3"
	extracted := extractDOI(doi)
	expected := "10.1010/a11-22\\(22\\)33v3"
	
	assert.EqualValues(t, expected, extracted)

	doiInvalid := "https://doi.org"
	extractedInvalid := extractDOI(doiInvalid)
	expectedInvalid := doiInvalid

	assert.EqualValues(t, expectedInvalid, extractedInvalid)
}

func TestDOISearch(t *testing.T) {

	mocks.GetDoFunc = func(req *http.Request) (*http.Response, error) {
		// Create a reader with the GET json response
		r := io.NopCloser(bytes.NewReader([]byte(epmcRespJson)))
		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostToDOISearch(c)

	DOISearch(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp PMCCoreResponse
	json.Unmarshal(bodyBytes, &testResp)

	assert.EqualValues(t, 1, int(testResp.HitCount))
	assert.EqualValues(t, "0000000", testResp.ResultList["result"][0].ID)
}

func TestFieldSearch(t *testing.T) {

	mocks.GetDoFunc = func(req *http.Request) (*http.Response, error) {
		// Create a reader with the GET json response
		r := io.NopCloser(bytes.NewReader([]byte(epmcRespJson)))
		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostToFieldSearch(c)

	FieldSearch(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp PMCCoreResponse
	json.Unmarshal(bodyBytes, &testResp)

	assert.EqualValues(t, 1, int(testResp.HitCount))
	assert.EqualValues(t, "0000000", testResp.ResultList["result"][0].ID)
	assert.Contains(t, testResp.Aggregations, "startDate")
	assert.Contains(t, testResp.Aggregations, "endDate")

	startDate := testResp.Aggregations["startDate"].(map[string]interface{})["value_as_string"].(string)
	assert.EqualValues(t, startDate, "2020-01-01T00:00:00Z")
	endDate := testResp.Aggregations["endDate"].(map[string]interface{})["value_as_string"].(string)
	assert.EqualValues(t, endDate, "2020-01-01T00:00:00Z")
}

func TestArrayFieldSearch(t *testing.T) {

	mocks.GetDoFunc = func(req *http.Request) (*http.Response, error) {
		// Create a reader with the GET json response
		r := io.NopCloser(bytes.NewReader([]byte(epmcRespJson)))
		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	w := httptest.NewRecorder()
	c := GetTestGinContext(w)
	MockPostToArrayFieldSearch(c)

	ArrayFieldSearch(c)

	assert.EqualValues(t, http.StatusOK, w.Code)

	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp PMCCoreResponse
	json.Unmarshal(bodyBytes, &testResp)

	assert.EqualValues(t, 2, int(testResp.HitCount))
	assert.EqualValues(t, "0000000", testResp.ResultList["result"][0].ID)
	assert.Contains(t, testResp.Aggregations, "startDate")
	assert.Contains(t, testResp.Aggregations, "endDate")
}

func TestBuildQueryString(t *testing.T) {
	query := FieldQuery{
		QueryString: "A Very Useful Dataset (AVUD)",
		Field : []string{"TITLE","ABSTRACT","METHODS"},
		Filters: map[string]map[string]interface{}{
			"paper": {
				"publicationDate": []interface{}{"2020", "2021"},
				"publicationType": []interface{}{"Review articles", "Preprints"},
			},
		},
	}
	
	queryString := buildQueryString(query)
	
	assert.Contains(t, queryString, "TITLE")
	assert.Contains(t, queryString, "PUB_TYPE:REVIEW")
	assert.Contains(t, queryString, "SRC:PPR")
	assert.Contains(t, queryString, "PUB_YEAR:[2020%20TO%202021]")
}

func TestBuildDoiQuery(t *testing.T) {
	query := Query{
		QueryString: "https://doi.org/10.3310/abcde",
		Filters: map[string]map[string]interface{}{
			"paper": {
				"publicationDate": []interface{}{"2020", "2021"},
				"publicationType": []interface{}{"Review articles", "Preprints"},
			},
		},
	}
	
	queryString := buildDoiQuery(query)
	
	assert.Contains(t, queryString, "DOI:10.3310/abcde")
	assert.Contains(t, queryString, "PUB_TYPE:REVIEW")
	assert.Contains(t, queryString, "SRC:PPR")
	assert.Contains(t, queryString, "PUB_YEAR:[2020%20TO%202021]")
}

func TestCalculateAggregations(t *testing.T) {
	pmcCore := PMCCoreResponse{
		ResultList: map[string][]PaperCore{
			"result": {
				{PubYear: "2020"},
				{PubYear: "2002"},
				{PubYear: "24"},
			},
		},
	}

	aggregations := calculateAggregations(pmcCore)

	assert.Contains(t, aggregations, "startDate")
	assert.Contains(t, aggregations, "endDate")
	startDate := aggregations["startDate"].(gin.H)["value_as_string"].(string)
	assert.EqualValues(t, startDate, "2002-01-01T00:00:00Z")
	// Assert end date is "2020" which is latest valid end date passed
	endDate := aggregations["endDate"].(gin.H)["value_as_string"].(string)
	assert.EqualValues(t, endDate, "2020-01-01T00:00:00Z")
}

func TestShuffleArrays(t *testing.T) {
	a := []string{"A", "B", "C"}
	b := []string{"A"}
	c := []string{"A", "B"}

	input := [][]string{a, b, c}

	shuffled := shuffleArrays(input)
	expected := []string{"A", "A", "A", "B", "B", "C"}

	assert.EqualValues(t, expected, shuffled)
}
