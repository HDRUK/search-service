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
// a file of synonyms called hdr_synonyms_set, which must be present _synonyms
// collection of the elastic deployment.
func DefineDatasetSettings(c *gin.Context) {
	// Elastic requires the index to be closed before settings are updated
	closeIndexByName("dataset")

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
		Index:      []string{"dataset"},
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
	openIndexByName("dataset")

	c.JSON(http.StatusOK, resp)
}

// DefineDatasetMappings initialises the datasets index and defines the custom
// mappings for specific fields which need to be used as filters.
// Mappings can only be defined BEFORE any data is indexed, updating mappings 
// requires reindexing.
func DefineDatasetMappings(c *gin.Context) {
	var buf bytes.Buffer
	elasticMappings := gin.H{
		"mappings": gin.H{
			"properties": gin.H{
				"publisherName": gin.H{"type": "keyword"},
				"dataProvider": gin.H{"type": "keyword"},
				"dataUseTitles": gin.H{"type": "keyword"},
				"collectionName": gin.H{"type": "keyword"},
				"geographicLocation": gin.H{"type": "keyword"},
				"accessService": gin.H{"type": "keyword"},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(elasticMappings); err != nil {
		fmt.Println(err.Error())
	}

	request := esapi.IndicesCreateRequest{
		Index:      "dataset",
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

	c.JSON(http.StatusOK, resp)
}

// DefineToolSettings updates the settings of the tools index in elastic to use
// a custom similarity scoring algorithm.  The mappings of the tools index are 
// updated so that the custom similarity algorithm is applied to the description
// field of the tools data. 
func DefineToolSettings(c *gin.Context) {
	// Elastic requires the index to be closed before settings are updated
	closeIndexByName("tool")

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
		Index:      []string{"tool"},
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
		Index:      []string{"tool"},
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
	openIndexByName("tool")

	c.JSON(http.StatusOK, gin.H{"acknowledged": true})
}

// DefineToolMappings initialises the tool index and defines the custom
// mappings for specific fields which need to be used as filters.
// Mappings can only be defined BEFORE any data is indexed, updating mappings 
// requires reindexing.
func DefineToolMappings(c *gin.Context) {
	var buf bytes.Buffer
	elasticMappings := gin.H{
		"mappings": gin.H{
			"properties": gin.H{
				"dataProvider": gin.H{"type": "keyword"},
				"license": gin.H{"type": "keyword"},
				"datasetTitles": gin.H{"type": "keyword"},
				"programmingLanguages": gin.H{"type": "keyword"},
				"typeCategory": gin.H{"type": "keyword"},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(elasticMappings); err != nil {
		fmt.Println(err.Error())
	}

	request := esapi.IndicesCreateRequest{
		Index:      "tool",
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

	c.JSON(http.StatusOK, resp)
}


// DefineCollectionSettings updates the settings of the collections index in elastic to use
// a custom similarity scoring algorithm.  The mappings of the collections index are 
// updated so that the custom similarity algorithm is applied to the description
// field of the collection data. 
func DefineCollectionSettings(c *gin.Context) {
	// Elastic requires the index to be closed before settings are updated
	closeIndexByName("collection")

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
		Index:      []string{"collection"},
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
		Index:      []string{"collection"},
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
	openIndexByName("collection")

	c.JSON(http.StatusOK, gin.H{"acknowledged": true})
}


// DefineCollectionMappings initialises the collection index and defines the custom
// mappings for specific fields which need to be used as filters.
// Mappings can only be defined BEFORE any data is indexed, updating mappings 
// requires reindexing.
func DefineCollectionMappings(c *gin.Context) {
	var buf bytes.Buffer
	elasticMappings := gin.H{
		"mappings": gin.H{
			"properties": gin.H{
				"publisherName": gin.H{"type": "keyword"},
				"dataProvider": gin.H{"type": "keyword"},
				"datasetTitles": gin.H{"type": "keyword"},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(elasticMappings); err != nil {
		fmt.Println(err.Error())
	}

	request := esapi.IndicesCreateRequest{
		Index:      "collection",
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

	c.JSON(http.StatusOK, resp)
}

// DefineDataUseMappings initialises the datauseregister index and defines the custom
// mappings for specific fields which need to be used as filters.
// Mappings can only be defined BEFORE any data is indexed, updating mappings 
// requires reindexing.
func DefineDataUseMappings(c *gin.Context) {
	var buf bytes.Buffer
	elasticMappings := gin.H{
		"mappings": gin.H{
			"properties": gin.H{
				"publisherName": gin.H{"type": "keyword"},
				"dataProvider": gin.H{"type": "keyword"},
				"sector": gin.H{"type": "keyword"},
				"organisationName": gin.H{"type": "keyword"},
				"datasetTitles": gin.H{"type": "keyword"},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(elasticMappings); err != nil {
		fmt.Println(err.Error())
	}

	request := esapi.IndicesCreateRequest{
		Index:      "datauseregister",
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

	c.JSON(http.StatusOK, resp)
}

// DefinePublicationMappings initialises the publication index and defines the custom
// mappings for specific fields which need to be used as filters.
// Mappings can only be defined BEFORE any data is indexed, updating mappings 
// requires reindexing.
func DefinePublicationMappings(c *gin.Context) {
	var buf bytes.Buffer
	elasticMappings := gin.H{
		"mappings": gin.H{
			"properties": gin.H{
				"publicationType": gin.H{"type": "keyword"},
				"datasetTitles": gin.H{"type": "keyword"},
				"publicationDate": gin.H{"type": "date"},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(elasticMappings); err != nil {
		fmt.Println(err.Error())
	}

	request := esapi.IndicesCreateRequest{
		Index:      "publication",
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

	c.JSON(http.StatusOK, resp)
}

// DefineDataProviderMappings initialises the dataprovider index and defines the custom
// mappings for specific fields which need to be used as filters.
// Mappings can only be defined BEFORE any data is indexed, updating mappings 
// requires reindexing.
func DefineDataProviderMappings(c *gin.Context) {
	var buf bytes.Buffer
	elasticMappings := gin.H{
		"mappings": gin.H{
			"properties": gin.H{
				"geographicLocation": gin.H{"type": "keyword"},
				"datasetTitles": gin.H{"type": "keyword"},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(elasticMappings); err != nil {
		fmt.Println(err.Error())
	}

	request := esapi.IndicesCreateRequest{
		Index:      "dataprovider",
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

	c.JSON(http.StatusOK, resp)
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
