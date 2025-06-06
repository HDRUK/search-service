package mocks

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
)

// MockClient is the mock client
type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

var (
	// PostDoFunc fetches the mock client's `Do` func for POST requests
	PostDoFunc func(req *http.Request) (*http.Response, error)
	// GetDoFunc fetches the mock client's `Do` func for GET requests
	GetDoFunc func(req *http.Request) (*http.Response, error)
)

// Do is the mock client's `Do` func
func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	if req.Method == "POST" {
		return PostDoFunc(req)
	} else {
		return GetDoFunc(req)
	}
}

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
	var responseBody string
	mocktrans.RoundTripFn = func(req *http.Request) (*http.Response, error) {
		if req.Method == "PUT" {
			responseBody = `{"acknowledged": true}`
		} else {
			responseBody = `{
				"took": 3,
				"timed_out": false,
				"_shards": {},
				"hits": {
					"hits": []
				},
				"aggregations": {}
			}`
		}
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
