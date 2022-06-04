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
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		log.Fatal(err)
	}
	log.Printf("Connected to MongoDB at %s", os.Getenv("MONGO_URI"))

	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")
	usersCollection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("users")

	// redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "",
		DB:       0,
	})
	status := redisClient.Ping(ctx)
	if status.Err() != nil {
		log.Fatal(status.Err())
	}
	log.Printf("redisClient at %s with status %v\n", os.Getenv("REDIS_ADDR"), status)

	recipesHandler = handlers.NewRecipesHandler(ctx, collection, redisClient)
	authHandler = handlers.NewAuthHAndler(usersCollection, ctx)
}

func main() {
	router := gin.Default()
	// // cookies
	// store, _ := redisStore.NewStore(10, "tcp", os.Getenv("REDIS_ADDR"), "", []byte("secret"))
	// router.Use(sessions.Sessions("recipes_api", store))
	// // end of cookie //

	// router.GET("/login", authHandler.SignInHandlerCookie)     // Cookie
	// router.POST("/signout", authHandler.SignOutHandlerCookie) // Cookie
	router.GET("/login", authHandler.SignInHandlerJWT) // JWT
	router.POST("/adduser", authHandler.AddUser)       // for testing only - to create users

	nonauth := router.Group("/v1")
	nonauth.GET("/recipes", recipesHandler.ListRecipesHandler)

	authorized := router.Group("/v1")
	// authorized.Use(authHandler.AuthMiddlewareAuth0())  // Auth0
	// authorized.Use(authHandler.AuthMiddlewareCookie())  // Cookie
	authorized.Use(authHandler.AuthMiddlewareJWT()) // JWT
	{
		authorized.POST("/recipes", recipesHandler.NewRecipeHandler)
		authorized.PUT("/recipes/:id", recipesHandler.UpdateRecipeHandler)
		authorized.DELETE("/recipes/:id", recipesHandler.DelRecipeHandler)
		authorized.GET("/recipes/search", recipesHandler.SearchRecipesHandler)
		authorized.GET("/recipes/:id", recipesHandler.GetRecipeHandler)
		authorized.POST("/refresh", authHandler.RefreshHandler)
	}

	router.Run()
}
