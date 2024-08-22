package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
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

	allowCredentials, err := strconv.ParseBool(os.Getenv("CORS_ALLOW_CREDENTIALS"))
	if err != nil {
		fmt.Println("unable to determine \"CORS_ALLOW_CREDENTIALS\" value")
	}
	maxAge, err := strconv.Atoi(os.Getenv("CORS_MAX_AGE"))
	if err != nil {
		fmt.Println("unable to determine \"CORS_MAX_AGE\" value")
	}

	// Implement CORS into gin server
	router.Use(cors.New(cors.Config{
		AllowOrigins:     configFromEnv("CORS_ALLOW_ORIGINS"),
		AllowMethods:     configFromEnv("CORS_ALLOW_METHODS"),
		AllowHeaders:     configFromEnv("CORS_ALLOW_HEADERS"),
		ExposeHeaders:    configFromEnv("CORS_EXPOSE_HEADERS"),
		AllowCredentials: allowCredentials,
		MaxAge:           time.Duration(maxAge) * time.Hour,
	}))

	// Define generic search endpoint, searches across all available entities
	router.POST("/search", search.SearchGeneric)
	router.POST("/search/datasets", search.DatasetSearch)
	router.POST("/search/tools", search.ToolSearch)
	router.POST("/search/collections", search.CollectionSearch)
	router.POST("/search/dur", search.DataUseSearch)
	router.POST("/search/publications", search.PublicationSearch)
	router.POST("/search/data_providers", search.DataProviderSearch)
	router.POST("/settings/datasets", search.DefineDatasetSettings)
	router.POST("/settings/tools", search.DefineToolSettings)
	router.POST("/settings/collections", search.DefineCollectionSettings)
	router.POST("/mappings/datasets", search.DefineDatasetMappings)
	router.POST("/mappings/collections", search.DefineCollectionMappings)
	router.POST("/mappings/dur", search.DefineDataUseMappings)
	router.POST("/mappings/publications", search.DefinePublicationMappings)
	router.POST("/mappings/tools", search.DefineToolMappings)
	router.POST("/mappings/data_providers", search.DefineDataProviderMappings)
	router.POST("/filters", search.ListFilters)
	router.POST("/similar/datasets", search.SearchSimilarDatasets)

	router.POST("/search/federated_papers/doi", search.DOISearch)
	router.POST("/search/federated_papers/field_search", search.FieldSearch)

	router.Run(os.Getenv("SEARCHSERVICE_HOST"))
}

// configFromEnv - Pulls in and translates comma delimited strings from env file
// to set as CORS config
func configFromEnv(envName string) []string {
	var retVal []string
	value := os.Getenv(envName)

	if strings.Contains(",", value) {
		retVal = strings.Split(value, ",")
	} else {
		retVal = []string{value}
	}

	fmt.Printf("configuring router cors \"%s\" to: %+v\n", envName, retVal)

	return retVal
}
