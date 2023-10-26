package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"

	"hdruk/search-service/utils/elastic"
	"hdruk/search-service/utils/mocks"
)

func init() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	elastic.ElasticClient = mocks.MockElasticClient()
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

	fmt.Printf("response body: %s\n", w.Body)
	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	var testResp []interface{}
	ok := json.Unmarshal(bodyBytes, &testResp)
	if ok != nil {
		fmt.Printf("unmarshalling error: %s", ok.Error())
	} else {
		fmt.Print("no unmarshal error")
	}

	fmt.Printf("\ntestResp: %s\n", testResp)
}
