package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/ronenniv/Go-Gin-Auth/models"
	"github.com/rs/xid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthHandler struct {
	collection *mongo.Collection
	ctx        context.Context
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

var key *ecdsa.PrivateKey

func NewAuthHAndler(collection *mongo.Collection, ctx context.Context) *AuthHandler {
	return &AuthHandler{
		collection: collection,
		ctx:        ctx,
	}
}

func (h *AuthHandler) SignInHandlerCookie(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})
		return
	}

	sha := sha256.New()
	sha.Write([]byte(user.Password))
	// fetch from mongo
	filter := bson.M{"username": user.Username, "password": sha.Sum(nil)}
	cur := h.collection.FindOne(h.ctx, filter)
	if cur.Err() != nil {
		c.JSON(http.StatusUnauthorized, models.Message{Message: "Incorrect user or password"})
		return
	}

	// session cookie
	sessionToken := xid.New().String()
	session := sessions.Default(c)
	session.Set("username", user.Username)
	session.Set("token", sessionToken)
	session.Options(sessions.Options{MaxAge: 10 * 60}) // 10 minutes age
	session.Save()
	c.JSON(http.StatusOK, models.Message{Message: "User signed in"})
}

func (h *AuthHandler) AddUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})
		return
	}

	sha := sha256.New()
	sha.Write([]byte(user.Password))
	user.Password = string(sha.Sum(nil))

	// insert to mongo
	filter := bson.D{
		{"username", user.Username}}
	cur := h.collection.FindOne(h.ctx, filter)
	if cur.Err() == nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: "user already exit"})
		return
	} else if cur.Err() != mongo.ErrNoDocuments {
		c.JSON(http.StatusInternalServerError, models.Message{Message: cur.Err().Error()})
		return
	}
	insert := bson.D{
		{"username", user.Username},
		{"password", sha.Sum(nil)}}
	_, err := h.collection.InsertOne(h.ctx, insert)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, user.Username)
}

func (handler *AuthHandler) AuthMiddlewareCookie() gin.HandlerFunc {
	// Cookie session
	return func(c *gin.Context) {
		session := sessions.Default(c)
		sessionToken := session.Get("token")
		log.Printf("cookie username=%s", session.Get("username"))
		if sessionToken == nil {
			c.JSON(http.StatusForbidden, models.Message{Error: "Not logged in"})
			c.Abort()
		}
		c.Next()
	}
}

func (handler *AuthHandler) SignOutHandlerCookie(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.JSON(http.StatusOK, models.Message{Message: "Signed out"})
}
