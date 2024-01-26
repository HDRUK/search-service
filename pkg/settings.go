package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/gin-gonic/gin"
)

// DefineDatasetSettings updates the settings of the datasets index in elastic
// to use a custom analyzer.  This analyzer includes synonym analysis based on
// a file of synonyms called elastic_synonyms.csv, which must be present in the
// /usr/share/elasticsearch/config directory of the elastic deployment.
func DefineDatasetSettings(c *gin.Context) {
	// Elastic requires the index to be closed before settings are updated
	closeIndexByName("datasets")

	var buf bytes.Buffer
	elasticSettings := gin.H{
		"settings": gin.H{
			"index": gin.H{
				"analysis": gin.H{
					"analyzer": gin.H{
						"medterms_analyzer": gin.H{
							"tokenizer": "standard",
							"filter": []string{
								"lowercase",
								"stemmer",
								"medterms_synonyms",
							},
						},
					},
					"filter": gin.H{
						"medterms_synonyms": gin.H{
							"type": "synonym",
							"synonyms_set": "hdr_synonyms_set",
							"updateable": true,
						},
					},
				},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(elasticSettings); err != nil {
		fmt.Println(err.Error())
	}

	request := esapi.IndicesPutSettingsRequest{
		Index:      []string{"datasets"},
		Body:       &buf,
	}
	response, err := request.Do(context.TODO(), ElasticClient)
	if err != nil {
		c.JSON(response.StatusCode, gin.H{"message": err.Error()})
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println(err.Error())
	}
	var resp map[string]interface{}
	json.Unmarshal(body, &resp)

	// Reopen the index
	openIndexByName("datasets")

	c.JSON(http.StatusOK, resp)
}

// DefineToolSettings updates the settings of the tools index in elastic to use
// a custom similarity scoring algorithm.  The mappings of the tools index are 
// updated so that the custom similarity algorithm is applied to the description
// field of the tools data. 
func DefineToolSettings(c *gin.Context) {
	// Elastic requires the index to be closed before settings are updated
	closeIndexByName("tools")

	var buf bytes.Buffer
	elasticSettings := gin.H{
		"settings": gin.H{
			"index": gin.H{
				"similarity": gin.H{
					"custom_similarity": gin.H{
				  		"type": "BM25",
				  		"b": 0.1,
					},
				},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(elasticSettings); err != nil {
		fmt.Println(err.Error())
	}

	request := esapi.IndicesPutSettingsRequest{
		Index:      []string{"tools"},
		Body:       &buf,
	}
	response, err := request.Do(context.TODO(), ElasticClient)
	if err != nil {
		c.JSON(response.StatusCode, gin.H{"message": err.Error()})
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println(err.Error())
	}
	var resp map[string]interface{}
	json.Unmarshal(body, &resp)

	var mappings bytes.Buffer
	elasticMappings := gin.H{
		"properties" : gin.H{
			"description" : gin.H{
				"type" : "text", 
				"similarity" : "custom_similarity",
			},
		},
	}
	if err := json.NewEncoder(&mappings).Encode(elasticMappings); err != nil {
		fmt.Println(err.Error())
	}

	mappingsRequest := esapi.IndicesPutMappingRequest{
		Index:      []string{"tools"},
		Body:       &mappings,
	}
	mappingsResponse, err := mappingsRequest.Do(context.TODO(), ElasticClient)
	if err != nil {
		c.JSON(mappingsResponse.StatusCode, gin.H{"message": err.Error()})
		return
	}
	defer mappingsResponse.Body.Close()
	mappingsBody, err := io.ReadAll(mappingsResponse.Body)
	if err != nil {
		fmt.Println(err.Error())
	}
	var mappingResp map[string]interface{}
	json.Unmarshal(mappingsBody, &mappingResp)

	// Reopen the index
	openIndexByName("tools")

	c.JSON(http.StatusOK, gin.H{"acknowledged": true})
}

// closeIndexByName closes the elastic index matching the provided name.
func closeIndexByName(indexName string) {
	closeIndexRequest := esapi.IndicesCloseRequest{
		Index: []string{indexName},
	}
	closeResponse, err := closeIndexRequest.Do(context.TODO(), ElasticClient)
	if err != nil {
		fmt.Printf("Error closing %s index: %s", indexName, err)
	}
	defer closeResponse.Body.Close()
}

// openIndexByName opens the elastic index matching the provided name.
func openIndexByName(indexName string) {
	openIndexRequest := esapi.IndicesOpenRequest{
		Index: []string{indexName},
	}
	openResponse, err := openIndexRequest.Do(context.TODO(), ElasticClient)
	if err != nil {
		fmt.Printf("Error opening %s index: %s", indexName, err)
	}
	defer openResponse.Body.Close()
}