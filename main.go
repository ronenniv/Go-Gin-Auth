package main

import (
	"context"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/ronenniv/Go-Gin-Auth/handlers"
	"github.com/ronenniv/Go-Gin-Auth/logger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

var recipesHandler *handlers.RecipesHandler
var authHandler *handlers.AuthHandler
var ginLogger *zap.Logger

func init() {
	ctx := context.Background()

	ginLogger = logger.InitLogger()

	// mongodb
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		ginLogger.Fatal("Cannot connect to MongoDB", zap.Error(err))
	}
	ginLogger.Info("Connected to MongoDB at URI", zap.String("MONGO_URI", os.Getenv("MONGO_URI")))
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		ginLogger.Fatal("Cannot ping to mongo", zap.Error(err))
	}
	ginLogger.Info("Pinged to MongoDB", zap.String("MONGO_URI", os.Getenv("MONGO_URI")))

	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")
	usersCollection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("users")

	// redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "",
		DB:       0,
	})
	status := redisClient.Ping(ctx)
	ginLogger.Info("rediClient Ping", zap.String("REDIS_ADDR", os.Getenv("REDIS_ADDR")), zap.Any("status", *status))

	recipesHandler = handlers.NewRecipesHandler(ctx, collection, redisClient, ginLogger)
	authHandler = handlers.NewAuthHAndler(usersCollection, ctx, ginLogger)
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(ginzap.Ginzap(logger.InitLogger(), time.RFC850, true)) // middleware for logging with Zap
	router.Use(cors.Default())
	{
		router.POST("/login", authHandler.SignInHandlerJWT) // JWT
		router.POST("/adduser", authHandler.AddUser)        // for testing only - to create users
	}
	admin := router.Group("")
	admin.Use(authHandler.AuthMiddlewareJWT()) // JWT
	{
		admin.POST("/refresh", authHandler.RefreshHandler)
		admin.POST("/logout", authHandler.LogoutHandlerJWT)
	}
	authorized := router.Group("/v1")
	authorized.Use(authHandler.AuthMiddlewareJWT()) // JWT
	{
		authorized.POST("/recipes", recipesHandler.NewRecipeHandler)
		authorized.PUT("/recipes/:id", recipesHandler.UpdateRecipeHandler)
		authorized.DELETE("/recipes/:id", recipesHandler.DelRecipeHandler)
		authorized.GET("/recipes", recipesHandler.ListRecipesHandler)
		authorized.GET("/recipes/search", recipesHandler.SearchRecipesHandler)
		authorized.GET("/recipes/:id", recipesHandler.GetRecipeHandler)
	}
	router.Run()
}
