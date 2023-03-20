package main

import (
	"context"
	"net/url"
	"os"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	adapter "github.com/gwatts/gin-adapter"
	"github.com/redis/go-redis/v9"
	"github.com/ronenniv/Go-Gin-Auth/handlers"
	"github.com/ronenniv/Go-Gin-Auth/logger"
	"github.com/ronenniv/Go-Gin-Auth/types"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

var recipesHandler *handlers.RecipesHandler
var authHandler *handlers.AuthHandler
var ginLogger *zap.Logger

func init() {
	// logger init.
	ginLogger = logger.InitLogger()

	// mongodb init.
	ctx, cancel := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		ginLogger.Fatal("mongo.Connect", zap.Error(err))
	}

	ctx, cancel2 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel2()
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		ginLogger.Fatal("client.Ping", zap.Error(err))
	}
	ginLogger.Info("Connected to MongoDB", zap.String("MONGO_URI", os.Getenv("MONGO_URI")))

	collection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")
	usersCollection := client.Database(os.Getenv("MONGO_DATABASE")).Collection("users")

	// redis init.
	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "",
		DB:       0,
	})

	ctx, cancel3 := context.WithTimeout(context.Background(), types.RedisCtxTimeout)
	defer cancel3()
	status := redisClient.Ping(ctx)
	if status.Err() != nil {
		ginLogger.Fatal("redisClient.Ping", zap.Error(status.Err()))
	}
	ginLogger.Info("redisClient connected", zap.String("REDIS_ADDR", os.Getenv("REDIS_ADDR")), zap.Any("status", status))

	recipesHandler = handlers.NewRecipesHandler(collection, redisClient, ginLogger)
	authHandler = handlers.NewAuthHAndler(usersCollection, ginLogger)
}

func main() {
	router := gin.Default()
	router.Use(ginzap.Ginzap(logger.InitLogger(), time.RFC850, true)) // middleware for logging with Zap
	router.Use(cors.Default())                                        // middleware to allows all origins
	{
		router.POST("/adduser", authHandler.AddUser) // for testing only - to create users
		group := router.Group("/v1")

		// this code copied from https://blog.sivamuthukumar.com/auth0-jwt-middleware-in-go-gin-web-framework
		issuerURL, err := url.Parse("https://" + os.Getenv("AUTH0_DOMAIN") + "/")
		if err != nil {
			ginLogger.Fatal("Failed to parse the issuer url", zap.Error(err))
		}

		provider := jwks.NewCachingProvider(issuerURL, types.Auth0CtxTimeout)

		jwtValidator, _ := validator.New(provider.KeyFunc,
			validator.RS256,
			issuerURL.String(),
			[]string{os.Getenv("AUTH0_AUDIENCE")},
		)

		jwtMiddleware := jwtmiddleware.New(jwtValidator.ValidateToken)
		group.Use(adapter.Wrap(jwtMiddleware.CheckJWT))
		// group.Use(authHandler.CheckJWT())
		{
			group.POST("/recipes", recipesHandler.NewRecipeHandler)
			group.PUT("/recipes/:id", recipesHandler.UpdateRecipeHandler)
			group.DELETE("/recipes/:id", recipesHandler.DelRecipeHandler)
			group.GET("/recipes/search", recipesHandler.SearchRecipesHandler)
			group.GET("/recipes/:id", recipesHandler.GetRecipeHandler)
			group.GET("/recipes", recipesHandler.ListRecipesHandler)
		}
	}

	if err := router.Run(); err != nil {
		os.Exit(1)
	}
}
