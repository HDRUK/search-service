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
	router.GET("/search", search.SearchGeneric)
	router.POST("/settings/datasets", search.DefineDatasetSettings)

	router.Run(os.Getenv("SEARCHSERVICE_HOST"))
}
