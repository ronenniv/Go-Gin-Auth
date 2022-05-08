package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
)

type Recipe struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Tags         []string  `json:"tags"`
	Ingredients  []string  `json:"ingredients"`
	Instructions []string  `json:"instructions"`
	PublishedAt  time.Time `json:"publishedAt"`
}

type Message struct {
	Message string `json:"error"`
}

var recipes []Recipe

func NewRecipeHandler(c *gin.Context) {
	var recipe Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, Message{Message: err.Error()})
		return
	}
	recipe.ID = xid.New().String()
	recipe.PublishedAt = time.Now()
	recipes = append(recipes, recipe)
	c.JSON(http.StatusOK, recipe)
}

func ListRecipesHandler(c *gin.Context) {
	if recipes == nil {
		c.JSON(http.StatusNotFound, Message{Message: "No recipes"})
		return
	}
	c.JSON(http.StatusOK, recipes)
}

func UpdateRecipeHandler(c *gin.Context) {
	id := c.Param("id")
	var recipe Recipe
	if err := c.ShouldBindJSON(&recipe); err != nil {
		c.JSON(http.StatusBadRequest, Message{Message: err.Error()})
		return
	}
	recipe.ID = id
	found := false
	for i, r := range recipes {
		if r.ID == recipe.ID {
			recipes[i] = recipe
			found = true
			break
		}
	}
	if found {
		c.JSON(http.StatusOK, recipe)
	} else {
		c.JSON(http.StatusNotFound, Message{Message: id + " not found"})
	}
}

func DelRecipeHandler(c *gin.Context) {
	id := c.Param("id")
	found := false
	recipe := Recipe{}
	for i, r := range recipes {
		if r.ID == id {
			recipe = r
			recipes = append(recipes[:i], recipes[i+1:]...)
			found = true
			break
		}
	}
	if found {
		c.JSON(http.StatusOK, recipe)
	} else {
		c.JSON(http.StatusNotFound, Message{Message: id + " not found"})
	}
}

func SearchRecipesHandler(c *gin.Context) {
	tag := c.Query("tag")
	listReciepes := make([]Recipe, 0)
	found := false
	for _, r := range recipes {
		for i := 0; i < len(r.Tags); i++ {
			if r.Tags[i] == tag {
				found = true
				listReciepes = append(listReciepes, r)
				break
			}
		}
	}
	if found {
		c.JSON(http.StatusOK, listReciepes)
	} else {
		c.JSON(http.StatusNotFound, Message{Message: tag + " not found"})
	}
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
