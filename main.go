package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/ronenniv/webclient/handlers"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var recipesHandler *handlers.RecipesHandler
var authHandler *handlers.AuthHandler

func init() {
	ctx := context.Background()

	// mongodb
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://admin:password@localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")

	collection := client.Database("demo").Collection("recipes")

	// redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	status := redisClient.Ping(ctx)
	log.Printf("redisClient status %v\n", status)

	recipesHandler = handlers.NewRecipesHandler(ctx, collection, redisClient)
	authHandler = &handlers.AuthHandler{}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-API-KEY") != os.Getenv("X_API_KEY") {
			c.AbortWithStatus(401)
		}
		c.Next()
	}
}

func main() {
	router := gin.Default()
	router.POST("/login", authHandler.SignInHandler)

	nonauth := router.Group("/v1")
	nonauth.GET("/recipes", recipesHandler.ListRecipesHandler)

	authorized := router.Group("/v1")
	authorized.Use(authHandler.AuthMiddleware())
	{
		// authorized.GET("/recipes", recipesHandler.ListRecipesHandler)
		authorized.POST("/recipes", recipesHandler.NewRecipeHandler)
		authorized.PUT("/recipes/:id", recipesHandler.UpdateRecipeHandler)
		authorized.DELETE("/recipes/:id", recipesHandler.DelRecipeHandler)
		authorized.GET("/recipes/search", recipesHandler.SearchRecipesHandler)
		authorized.GET("/recipes/:id", recipesHandler.GetRecipeHandler)
	}

	router.Run()
}
