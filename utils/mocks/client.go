package mocks

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
)

// Recommended method for mocking the elasticsearch client in tests
// See [go-elasticsearch](https://github.com/elastic/go-elasticsearch/tree/main#examples)
type MockTransport struct {
	Response    *http.Response
	RoundTripFn func(req *http.Request) (*http.Response, error)
}

func (mt *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return mt.RoundTripFn(req)
}

func MockElasticClient() *elasticsearch.Client {
	mocktrans := MockTransport{}
	mocktrans.RoundTripFn = func(req *http.Request) (*http.Response, error) {
		responseBody := `{
			"took": 3,
			"timed_out": false,
			"_shards": {},
			"hits": {}	
		}`
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
		}
		return resp, nil
	}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Transport: &mocktrans,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
	return client
}
