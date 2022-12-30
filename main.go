package main

import (
	"context"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	redisStore "github.com/gin-contrib/sessions/redis"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/ronenniv/Go-Gin-Auth/handlers"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

var recipesHandler *handlers.RecipesHandler
var authHandler *handlers.AuthHandler
var logger, _ = zap.NewProduction()

func init() {
	ctx := context.Background()

	// mongodb
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		logger.Fatal("Cannot conenct to mongo", zap.Error(err))
	}
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		logger.Fatal("Cannot ping to mongo", zap.Error(err))
	}
	logger.Info("Connected to MongoDB", zap.String("MONGO_URI", os.Getenv("MONGO_URI")))

	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")
	usersCollection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("users")

	// redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "",
		DB:       0,
	})
	status := redisClient.Ping(ctx)
	logger.Info("rediClinet Ping", zap.String("REDIS_ADDR", os.Getenv("REDIS_ADDR")), zap.Any("status", *status))

	recipesHandler = handlers.NewRecipesHandler(ctx, collection, redisClient, logger)
	authHandler = handlers.NewAuthHAndler(usersCollection, ctx, logger)
}

func main() {
	router := gin.Default()
	router.Use(ginzap.Ginzap(logger, time.RFC850, true))
	router.Use(cors.Default())
	// cookies
	store, _ := redisStore.NewStore(10, "tcp", os.Getenv("REDIS_ADDR"), "", []byte("secret"))
	router.Use(sessions.Sessions("recipes_api", store))
	// end of cookie //
	{
		router.POST("/login", authHandler.SignInHandlerCookie)    // Cookie
		router.POST("/signout", authHandler.SignOutHandlerCookie) // Cookie
		router.POST("/adduser", authHandler.AddUser)              // for testing only - to create users
	}
	authorized := router.Group("/v1")
	authorized.Use(authHandler.AuthMiddlewareCookie()) // Cookie
	{
		authorized.POST("/recipes", recipesHandler.NewRecipeHandler)
		authorized.PUT("/recipes/:id", recipesHandler.UpdateRecipeHandler)
		authorized.DELETE("/recipes/:id", recipesHandler.DelRecipeHandler)
		authorized.GET("/recipes/search", recipesHandler.SearchRecipesHandler)
		authorized.GET("/recipes/:id", recipesHandler.GetRecipeHandler)
		authorized.GET("/recipes", recipesHandler.ListRecipesHandler)
	}
	if err := router.Run(); err != nil {
		os.Exit(1)
	}
}
