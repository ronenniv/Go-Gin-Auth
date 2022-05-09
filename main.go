package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Recipe struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	Name         string             `json:"name" bson:"name"`
	Tags         []string           `json:"tags" bson:"tags"`
	Ingredients  []string           `json:"ingredients" bson:"ingredients"`
	Instructions []string           `json:"instructions" bson:"instructions"`
	PublishedAt  time.Time          `json:"publishedAt" bson:"publishedAt"`
}

type Message struct {
	Message string `json:"error"`
}

var collection *mongo.Collection
var ctx context.Context

// create new recipe with the body json request
func NewRecipeHandler(c *gin.Context) {
	var recipe Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, Message{Message: err.Error()})
		return
	}
	recipe.ID = primitive.NewObjectID()
	recipe.PublishedAt = time.Now()

	if _, err := collection.InsertOne(ctx, recipe); err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, Message{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, recipe)
}

// provide list of all recipes
func ListRecipesHandler(c *gin.Context) {
	cur, err := collection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusNotFound, Message{Message: "No recipes"})
		return
	}
	defer cur.Close(ctx)

	recipes := make([]Recipe, 0, cur.RemainingBatchLength())
	cur.All(ctx, &recipes)
	c.JSON(http.StatusOK, recipes)
}

// update single recipe for hte provided id and body details
func UpdateRecipeHandler(c *gin.Context) {
	id := c.Param("id")
	var recipe Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, Message{Message: err.Error()})
		return
	}
	objectid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, Message{Message: id + "is not a valid ObjectId"})
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
	result := collection.FindOneAndUpdate(ctx, filter, update, opts)
	if result.Err() != nil {
		if result.Err() == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, Message{Message: id + " is not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Message{Message: result.Err().Error()})
		return
	}
	c.JSON(http.StatusOK, recipe)
}

// delete recipe with the provided id
func DelRecipeHandler(c *gin.Context) {
	id := c.Param("id")
	objectid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, Message{Message: id + "is not a valid ObjectId"})
		return
	}
	res, err := collection.DeleteOne(ctx, bson.D{{"_id", objectid}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, Message{Message: err.Error()})
		return
	}
	if res.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, Message{Message: id + " not found"})
		return
	}
	c.JSON(http.StatusOK, id)
}

func SearchRecipesHandler(c *gin.Context) {
	tag := c.Query("tag")

	cur, err := collection.Find(ctx, bson.D{{"tags", tag}})
	if err != nil {
		c.JSON(http.StatusNotFound, Message{Message: "No recipes"})
		return
	}
	defer cur.Close(ctx)

	recipes := make([]Recipe, 0, cur.RemainingBatchLength())
	if err = cur.All(ctx, &recipes); err != nil {
		c.JSON(http.StatusInternalServerError, Message{Message: err.Error()})
		return
	}
	if len(recipes) > 0 {
		c.JSON(http.StatusOK, recipes)
	} else {
		c.JSON(http.StatusNotFound, Message{Message: tag + " not found"})
	}
}

func init() {
	ctx = context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://admin:password@localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")

	collection = client.Database("demo").Collection("recipes")
}

func main() {
	router := gin.Default()
	router.POST("/recipes", NewRecipeHandler)
	router.GET("/recipes", ListRecipesHandler)
	router.PUT("/recipes/:id", UpdateRecipeHandler)
	router.DELETE("/recipes/:id", DelRecipeHandler)
	router.GET("/recipes/search", SearchRecipesHandler)
	router.Run()
}
