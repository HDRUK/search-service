package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	search "hdruk/search-service/pkg"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	search.DefineElasticClient()

	router := gin.Default()
	// Define generic search endpoint, searches across all available entities
	router.GET("/search", search.SearchGeneric)

	router.Run("localhost:8080")
}
