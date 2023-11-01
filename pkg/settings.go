package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func DefineDatasetSettings(c *gin.Context) {
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
							"synonyms_path": "elastic_synonyms.csv",
							"updateable": true,
						},
					},
				},
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(elasticSettings); err != nil {
		log.Fatal(err.Error())
	}

	response, err := ElasticClient.Indices.PutSettings(&buf)
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Printf("response from elastic: %s", response)

	c.JSON(http.StatusOK, response.Body)
}