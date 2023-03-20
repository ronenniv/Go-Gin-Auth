package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/ronenniv/Go-Gin-Auth/models"
	"github.com/ronenniv/Go-Gin-Auth/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const redisTTL = time.Hour * 12 // Redis TTL 12 hours

type RecipesHandler struct {
	collection  *mongo.Collection
	redisClient *redis.Client
	logger      *zap.Logger
}

func NewRecipesHandler(collection *mongo.Collection, redisClient *redis.Client, logger *zap.Logger) *RecipesHandler {
	return &RecipesHandler{
		collection:  collection,
		redisClient: redisClient,
		logger:      logger,
	}
}

var ErrNoRecipesFound = errors.New("no recipes found")

// getAllRecipesFromMongo will get all recipes from Mongo
// it will update redis with the results
// and will return the slice with recipes
// err can be ErrNoRecipesFound, or any other unknown error.
func (rh *RecipesHandler) getAllRecipesFromMongo() ([]models.Recipe, error) {
	ctx, cancel1 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel1()

	cur, err := rh.collection.Find(ctx, bson.M{})
	if err != nil {
		rh.logger.Info("No recipes", zap.Error(err))

		return nil, ErrNoRecipesFound
	}

	ctx, cancel2 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel2()
	defer cur.Close(ctx)

	recipes := make([]models.Recipe, 0, cur.RemainingBatchLength())

	ctx, cancel3 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel3()

	if err := cur.All(ctx, &recipes); err != nil {
		rh.logger.Info("Cursor error", zap.Error(err))

		return nil, errors.New("cannot retrieve recipes")
	}

	// update redis with recipes
	data, _ := json.Marshal(recipes)

	ctx, cancel4 := context.WithTimeout(context.Background(), types.RedisCtxTimeout)
	defer cancel4()
	rh.redisClient.Set(ctx, "recipes", string(data), redisTTL)

	return recipes, nil
}

// provide list of all recipes.
func (rh *RecipesHandler) ListRecipesHandler(c *gin.Context) {
	var recipes []models.Recipe

	ctx, cancel := context.WithTimeout(context.Background(), types.RedisCtxTimeout)
	defer cancel()
	val, err := rh.redisClient.Get(ctx, "recipes").Result()
	if err != nil {
		// redis miss cache
		if errors.Is(err, redis.Nil) {
			rh.logger.Debug("redis miss cache")
			if recipes, err = rh.getAllRecipesFromMongo(); err != nil {
				if errors.Is(err, ErrNoRecipesFound) {
					c.JSON(http.StatusNotFound, models.Message{Message: "No recipes"})
				} else {
					c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
				}
			}
			c.JSON(http.StatusOK, recipes)

			return
		} else {
			// redis error
			rh.logger.Info("", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})

			return
		}
	}

	// hit cache. return result from cache
	rh.logger.Debug("redis hit cache")
	recipes = make([]models.Recipe, 0)
	if err := json.Unmarshal([]byte(val), &recipes); err != nil {
		rh.logger.Info("unmasrshal failed", zap.Error(err), zap.Any("recipes", recipes))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
	}

	c.JSON(http.StatusOK, recipes)
}

func (rh *RecipesHandler) SearchRecipesHandler(c *gin.Context) {
	tag := c.Query("tag")

	ctx, cancel1 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel1()
	cur, err := rh.collection.Find(ctx, bson.D{{"tags", tag}})
	if err != nil {
		rh.logger.Info("No recipes")
		c.JSON(http.StatusNotFound, models.Message{Message: "No recipes"})

		return
	}
	ctx, cancel2 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel2()
	defer cur.Close(ctx)

	recipes := make([]models.Recipe, 0, cur.RemainingBatchLength())

	ctx, cancel3 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel3()

	if err = cur.All(ctx, &recipes); err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})

		return
	}
	if len(recipes) == 0 {
		rh.logger.Info("not found", zap.String("", tag))
		c.JSON(http.StatusNotFound, models.Message{Message: tag + " not found"})

		return
	}

	c.JSON(http.StatusOK, recipes)
}

// delete recipe with the provided id.
func (rh *RecipesHandler) DelRecipeHandler(c *gin.Context) {
	id := c.Param("id")
	objectid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Message: id + "is not a valid ObjectId"})

		return
	}
	ctx, cancel1 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel1()
	res, err := rh.collection.DeleteOne(ctx, bson.D{{"_id", objectid}})
	if err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})

		return
	}
	if res.DeletedCount == 0 {
		rh.logger.Info("not found", zap.String("", id))
		c.JSON(http.StatusNotFound, models.Message{Message: id + " not found"})

		return
	}

	ctx, cancel2 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel2()
	rh.redisClient.Del(ctx, "recipes")

	ctx, cancel3 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel3()
	rh.redisClient.Del(ctx, id)
	rh.logger.Debug("remove id form redis", zap.String("", id))

	c.JSON(http.StatusOK, id)
}

// update single recipe for the provided id and body details.
func (rh *RecipesHandler) UpdateRecipeHandler(c *gin.Context) {
	id := c.Param("id")

	var recipe models.Recipe
	// get request body
	if err := c.ShouldBindJSON(&recipe); err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})

		return
	}
	// convert id to mongodb object id
	objectid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		rh.logger.Info("not a valid ObjectId", zap.String("objectId", objectid.String()), zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Message: id + "is not a valid ObjectId"})

		return
	}
	recipe.ID = objectid
	opts := options.FindOneAndUpdate().SetUpsert(false)
	filter := bson.D{{"_id", objectid}}
	update := bson.D{{"$set", bson.D{
		{"name", recipe.Name},
		{"instructions", recipe.Instructions},
		{"ingredients", recipe.Ingredients},
		{"tags", recipe.Tags},
	}}}

	ctx, cancel1 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel1()
	result := rh.collection.FindOneAndUpdate(ctx, filter, update, opts)
	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			rh.logger.Info("id not found", zap.String("", id))
			c.JSON(http.StatusNotFound, models.Message{Message: id + " is not found"})

			return
		}
		rh.logger.Info("", zap.Error(result.Err()))
		c.JSON(http.StatusInternalServerError, models.Message{Error: result.Err().Error()})

		return
	}

	ctx, cancel2 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel2()
	rh.redisClient.Del(ctx, "recipes")
	rh.logger.Debug("Remove recipes from Redis")

	data, _ := json.Marshal(&recipe)
	ctx, cancel3 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel3()
	rh.redisClient.Set(ctx, id, string(data), redisTTL)
	rh.logger.Debug("update redis", zap.String("id", id))

	c.JSON(http.StatusOK, recipe)
}

// get single recipe for the provided id.
func (rh *RecipesHandler) GetRecipeHandler(c *gin.Context) {
	var recipe models.Recipe
	id := c.Param("id")

	// check if id exist in redis
	ctx, cancel1 := context.WithTimeout(context.Background(), types.RedisCtxTimeout)
	defer cancel1()
	res, err := rh.redisClient.Get(ctx, id).Result()
	if errors.Is(err, redis.Nil) {
		rh.logger.Debug("redis cache miss", zap.String("id", id))
		// convert request id to mongodb objectid
		objectid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			rh.logger.Info("not a valid ObjectId", zap.String("objectId", objectid.String()), zap.Error(err))
			c.JSON(http.StatusBadRequest, models.Message{Message: id + "is not a valid ObjectId"})

			return
		}

		// fetch from mongo
		filter := bson.D{{"_id", objectid}}
		ctx, cancel2 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
		defer cancel2()
		err = rh.collection.FindOne(ctx, filter).Decode(&recipe)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				rh.logger.Info("id not found", zap.String("id", id))
				c.JSON(http.StatusNotFound, models.Message{Message: id + " is not found"})

				return
			}

			rh.logger.Info("error in FindOne", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})

			return
		}
		// add entry to redis
		data, _ := json.Marshal(&recipe)
		ctx, cancel3 := context.WithTimeout(context.Background(), types.RedisCtxTimeout)
		defer cancel3()
		rh.redisClient.Set(ctx, id, string(data), redisTTL)
		rh.logger.Debug("add to redis", zap.ByteString("data", data))
	} else {
		// id exist in redis
		rh.logger.Debug("redis cach hit", zap.String("id", id))
		if err = json.Unmarshal([]byte(res), &recipe); err != nil {
			rh.logger.Info("", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})

			return
		}
	}

	c.JSON(http.StatusOK, recipe)
}

// create new recipe with the body json request.
func (rh *RecipesHandler) NewRecipeHandler(c *gin.Context) {
	var recipe models.Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})

		return
	}
	recipe.ID = primitive.NewObjectID()
	recipe.PublishedAt = time.Now()

	ctx, cancel1 := context.WithTimeout(context.Background(), types.RedisCtxTimeout)
	defer cancel1()
	if _, err := rh.collection.InsertOne(ctx, recipe); err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})

		return
	}

	ctx, cancel2 := context.WithTimeout(context.Background(), types.RedisCtxTimeout)
	defer cancel2()
	rh.redisClient.Del(ctx, "recipes")
	rh.logger.Debug("Remove recipes from Redis")
	id, _ := recipe.ID.MarshalText()
	data, _ := json.Marshal(&recipe)

	ctx, cancel3 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel3()
	rh.redisClient.Set(ctx, string(id), string(data), redisTTL)
	rh.logger.Debug("add to redis", zap.ByteString("id", id))

	c.JSON(http.StatusOK, recipe)
}
