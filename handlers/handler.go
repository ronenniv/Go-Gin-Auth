package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/ronenniv/Go-Gin-Auth/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RecipesHandler struct {
	collection  *mongo.Collection
	ctx         context.Context
	redisClient *redis.Client
}

func NewRecipesHandler(ctx context.Context, collection *mongo.Collection, redisClient *redis.Client) *RecipesHandler {
	return &RecipesHandler{
		collection:  collection,
		ctx:         ctx,
		redisClient: redisClient,
	}
}

// provide list of all recipes
func (rh *RecipesHandler) ListRecipesHandler(c *gin.Context) {
	val, err := rh.redisClient.Get(rh.ctx, "recipes").Result()
	if err == redis.Nil {
		log.Printf("redis miss cache")
		cur, err := rh.collection.Find(rh.ctx, bson.M{})
		if err != nil {
			c.JSON(http.StatusNotFound, models.Message{Message: "No recipes"})
			return
		}
		defer cur.Close(rh.ctx)

		recipes := make([]models.Recipe, 0, cur.RemainingBatchLength())
		cur.All(rh.ctx, &recipes)

		// update redis with recipes
		data, _ := json.Marshal(recipes)
		rh.redisClient.Set(rh.ctx, "recipes", string(data), 0)

		c.JSON(http.StatusOK, recipes)
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})
		return
	} else {
		log.Println("redis hit cache")
		recipes := make([]models.Recipe, 0)
		json.Unmarshal([]byte(val), &recipes)
		c.JSON(http.StatusOK, recipes)
	}
}

func (rh *RecipesHandler) SearchRecipesHandler(c *gin.Context) {
	tag := c.Query("tag")

	cur, err := rh.collection.Find(rh.ctx, bson.D{{"tags", tag}})
	if err != nil {
		c.JSON(http.StatusNotFound, models.Message{Message: "No recipes"})
		return
	}
	defer cur.Close(rh.ctx)

	recipes := make([]models.Recipe, 0, cur.RemainingBatchLength())
	if err = cur.All(rh.ctx, &recipes); err != nil {
		c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})
		return
	}
	if len(recipes) > 0 {
		c.JSON(http.StatusOK, recipes)
	} else {
		c.JSON(http.StatusNotFound, models.Message{Message: tag + " not found"})
	}
}

// delete recipe with the provided id
func (rh *RecipesHandler) DelRecipeHandler(c *gin.Context) {
	id := c.Param("id")
	objectid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: id + "is not a valid ObjectId"})
		return
	}
	res, err := rh.collection.DeleteOne(rh.ctx, bson.D{{"_id", objectid}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})
		return
	}
	if res.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, models.Message{Message: id + " not found"})
		return
	}

	log.Printf("Remove %s from Redis", id)
	rh.redisClient.Del(rh.ctx, "recipes")
	rh.redisClient.Del(rh.ctx, id)

	c.JSON(http.StatusOK, id)
}

// update single recipe for the provided id and body details
func (rh *RecipesHandler) UpdateRecipeHandler(c *gin.Context) {
	id := c.Param("id")
	var recipe models.Recipe
	// get request body
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})
		return
	}
	// convert id to mongodb object id
	objectid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
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
			c.JSON(http.StatusNotFound, models.Message{Message: id + " is not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.Message{Message: result.Err().Error()})
		return
	}

	log.Println("Remove recipes from Redis")
	rh.redisClient.Del(rh.ctx, "recipes")
	log.Printf("Update %s in redis", id)
	data, _ := json.Marshal(&recipe)
	rh.redisClient.Set(rh.ctx, id, string(data), 0)

	c.JSON(http.StatusOK, recipe)
}

// get single recipe for the provided id
func (rh *RecipesHandler) GetRecipeHandler(c *gin.Context) {
	var recipe models.Recipe
	id := c.Param("id")

	// check if id exist in redis
	res, err := rh.redisClient.Get(rh.ctx, id).Result()
	if err == redis.Nil {
		log.Printf("%s redis cache miss", id)
		// convert request id to mongodb objectid
		objectid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Message{Message: id + "is not a valid ObjectId"})
			return
		}

		// fetch from mongo
		filter := bson.D{{"_id", objectid}}
		err = rh.collection.FindOne(rh.ctx, filter).Decode(&recipe)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusNotFound, models.Message{Message: id + " is not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})
			return
		}
		// add entry to redis
		log.Printf("Add %s to Redis", id)
		data, _ := json.Marshal(&recipe)
		rh.redisClient.Set(rh.ctx, id, string(data), 0)
	} else {
		// id exist in redis
		log.Printf("%s redis cache hit", id)
		if err = json.Unmarshal([]byte(res), &recipe); err != nil {
			c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, recipe)
}

// create new recipe with the body json request
func (rh *RecipesHandler) NewRecipeHandler(c *gin.Context) {
	var recipe models.Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})
		return
	}
	recipe.ID = primitive.NewObjectID()
	recipe.PublishedAt = time.Now()

	if _, err := rh.collection.InsertOne(rh.ctx, recipe); err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})
		return
	}

	log.Println("Remove recipes from Redis")
	rh.redisClient.Del(rh.ctx, "recipes")
	id, _ := recipe.ID.MarshalText()
	log.Printf("add %s to redis", id)
	data, _ := json.Marshal(&recipe)
	rh.redisClient.Set(rh.ctx, string(id), string(data), 0)

	c.JSON(http.StatusOK, recipe)
}
