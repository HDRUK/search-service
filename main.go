package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	search "hdruk/search-service/pkg"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Println("Could not load variables from .env.")
	}

	debugLogs := os.Getenv("DEBUG_LOGGING")
	if debugLogs == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	} else {
		slog.SetLogLoggerLevel(slog.LevelInfo)
	}

	search.DefineElasticClient()

	router := gin.Default()

	router.GET("/status", search.HealthCheck)

	// Define generic search endpoint, searches across all available entities
	router.POST("/search", search.SearchGeneric)
	router.POST("/search/datasets", search.DatasetSearch)
	router.POST("/search/tools", search.ToolSearch)
	router.POST("/search/collections", search.CollectionSearch)
	router.POST("/search/dur", search.DataUseSearch)
	router.POST("/search/publications", search.PublicationSearch)
	router.POST("/search/data_providers", search.DataProviderSearch)
	router.POST("/search/data_custodian_networks", search.DataCustodianNetworkSearch)

	router.POST("/settings/tools", search.DefineToolSettings)
	router.POST("/settings/collections", search.DefineCollectionSettings)
	router.POST("/settings/data_custodian_networks", search.DefineDataCustodianNetworkSettings)

	router.POST("/mappings/datasets", search.DefineDatasetMappings)
	router.POST("/mappings/collections", search.DefineCollectionMappings)
	router.POST("/mappings/dur", search.DefineDataUseMappings)
	router.POST("/mappings/publications", search.DefinePublicationMappings)
	router.POST("/mappings/tools", search.DefineToolMappings)
	router.POST("/mappings/data_providers", search.DefineDataProviderMappings)
	router.POST("/mappings/data_custodian_networks", search.DefineDataCustodianNetworkMappings)

	router.POST("/filters", search.ListFilters)
	router.POST("/similar/datasets", search.SearchSimilarDatasets)

	router.POST("/search/federated_papers/doi", search.DOISearch)
	router.POST("/search/federated_papers/field_search", search.FieldSearch)
	router.POST("/search/federated_papers/field_search/array", search.ArrayFieldSearch)

	router.Run(os.Getenv("SEARCHSERVICE_HOST"))
}
