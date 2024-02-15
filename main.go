package main

import (
	"fmt"
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

	search.DefineElasticClient()

	router := gin.Default()
	// Define generic search endpoint, searches across all available entities
	router.POST("/search", search.SearchGeneric)
	router.POST("/search/datasets", search.DatasetSearch)
	router.POST("/search/tools", search.ToolSearch)
	router.POST("/search/collections", search.CollectionSearch)
	router.POST("/search/dur", search.DataUseSearch)
	router.POST("/settings/datasets", search.DefineDatasetSettings)
	router.POST("/settings/tools", search.DefineToolSettings)
	router.POST("/mappings/datasets", search.DefineDatasetMappings)
	router.POST("/filters", search.ListFilters)

	router.Run(os.Getenv("SEARCHSERVICE_HOST"))
}
