package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/ronenniv/Go-Gin-Auth/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const redisTTL = time.Hour * 12 // Redis TTL 12 hours

type RecipesHandler struct {
	collection  *mongo.Collection
	ctx         context.Context
	redisClient *redis.Client
	logger      *zap.Logger
}

func NewRecipesHandler(ctx context.Context, collection *mongo.Collection, redisClient *redis.Client, logger *zap.Logger) *RecipesHandler {
	return &RecipesHandler{
		collection:  collection,
		ctx:         ctx,
		redisClient: redisClient,
		logger:      logger,
	}
}

// provide list of all recipes
func (rh *RecipesHandler) ListRecipesHandler(c *gin.Context) {
	val, err := rh.redisClient.Get(rh.ctx, "recipes").Result()
	if err == redis.Nil {
		rh.logger.Debug("redis miss cache")
		cur, err := rh.collection.Find(rh.ctx, bson.M{})
		if err != nil {
			rh.logger.Info("No recipes", zap.Error(err))
			c.JSON(http.StatusNotFound, models.Message{Error: "No recipes"})
			return
		}
		defer cur.Close(rh.ctx)

		recipes := make([]models.Recipe, 0, cur.RemainingBatchLength())
		if err := cur.All(rh.ctx, &recipes); err != nil {
			rh.logger.Info("", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
			return
		}

		// update redis with recipes
		data, _ := json.Marshal(recipes)
		rh.redisClient.Set(rh.ctx, "recipes", string(data), redisTTL)

		c.JSON(http.StatusOK, recipes)
	} else if err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
		return
	} else {
		rh.logger.Debug("redis hit cache")
		recipes := make([]models.Recipe, 0)
		if err := json.Unmarshal([]byte(val), &recipes); err != nil {
			rh.logger.Info("unmasrshal fialed", zap.Any("recipes", recipes))
			c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
		}
		c.JSON(http.StatusOK, recipes)
	}
}

func (rh *RecipesHandler) SearchRecipesHandler(c *gin.Context) {
	tag := c.Query("tag")

	cur, err := rh.collection.Find(rh.ctx, bson.D{{"tags", tag}})
	if err != nil {
		rh.logger.Info("No recipes")
		c.JSON(http.StatusNotFound, models.Message{Message: "No recipes"})
		return
	}
	defer cur.Close(rh.ctx)

	recipes := make([]models.Recipe, 0, cur.RemainingBatchLength())
	if err = cur.All(rh.ctx, &recipes); err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
		return
	}
	if len(recipes) == 0 {
		rh.logger.Info("not found", zap.String("", tag))
		c.JSON(http.StatusNotFound, models.Message{Message: tag + " not found"})
		return
	}
	c.JSON(http.StatusOK, recipes)
}

// delete recipe with the provided id
func (rh *RecipesHandler) DelRecipeHandler(c *gin.Context) {
	id := c.Param("id")
	objectid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Message: id + "is not a valid ObjectId"})
		return
	}
	res, err := rh.collection.DeleteOne(rh.ctx, bson.D{{"_id", objectid}})
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
	rh.redisClient.Del(rh.ctx, "recipes")
	rh.redisClient.Del(rh.ctx, id)
	rh.logger.Debug("remove id form redis", zap.String("", id))

	c.JSON(http.StatusOK, id)
}

// update single recipe for the provided id and body details
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
	result := rh.collection.FindOneAndUpdate(rh.ctx, filter, update, opts)
	if result.Err() != nil {
		if result.Err() == mongo.ErrNoDocuments {
			rh.logger.Info("id not found", zap.String("", id))
			c.JSON(http.StatusNotFound, models.Message{Message: id + " is not found"})
			return
		}
		rh.logger.Info("", zap.Error(result.Err()))
		c.JSON(http.StatusInternalServerError, models.Message{Error: result.Err().Error()})
		return
	}

	rh.redisClient.Del(rh.ctx, "recipes")
	rh.logger.Debug("Remove recipes from Redis")

	data, _ := json.Marshal(&recipe)
	rh.redisClient.Set(rh.ctx, id, string(data), redisTTL)
	rh.logger.Debug("update redis", zap.String("id", id))

	c.JSON(http.StatusOK, recipe)
}

// get single recipe for the provided id
func (rh *RecipesHandler) GetRecipeHandler(c *gin.Context) {
	var recipe models.Recipe
	id := c.Param("id")

	// check if id exist in redis
	res, err := rh.redisClient.Get(rh.ctx, id).Result()
	if err == redis.Nil {
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
		err = rh.collection.FindOne(rh.ctx, filter).Decode(&recipe)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				rh.logger.Info("id not found", zap.String("id", id))
				c.JSON(http.StatusNotFound, models.Message{Message: id + " is not found"})
				return
			}
			rh.logger.Info("", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
			return
		}
		// add entry to redis
		data, _ := json.Marshal(&recipe)
		rh.redisClient.Set(rh.ctx, id, string(data), redisTTL)
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

// create new recipe with the body json request
func (rh *RecipesHandler) NewRecipeHandler(c *gin.Context) {
	var recipe models.Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})
		return
	}
	recipe.ID = primitive.NewObjectID()
	recipe.PublishedAt = time.Now()

	if _, err := rh.collection.InsertOne(rh.ctx, recipe); err != nil {
		rh.logger.Info("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})
		return
	}

	rh.redisClient.Del(rh.ctx, "recipes")
	rh.logger.Debug("Remove recipes from Redis")
	id, _ := recipe.ID.MarshalText()
	data, _ := json.Marshal(&recipe)
	rh.redisClient.Set(rh.ctx, string(id), string(data), redisTTL)
	rh.logger.Debug("add to redis", zap.ByteString("id", id))

	c.JSON(http.StatusOK, recipe)
}
