package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"log"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go/v4"
	"github.com/gin-gonic/gin"
	"github.com/ronenniv/webclient/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthHandler struct {
	collection *mongo.Collection
	ctx        context.Context
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

type JWTOutput struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

var key *ecdsa.PrivateKey

func NewAuthHAndler(collection *mongo.Collection, ctx context.Context) *AuthHandler {
	return &AuthHandler{
		collection: collection,
		ctx:        ctx,
	}
}

func (h *AuthHandler) SignInHandler(c *gin.Context) {
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

	expirationTime := time.Now().Add(10 * time.Minute)
	claims := &Claims{
		Username: user.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: jwt.At(expirationTime),
		},
	}
	var err error
	key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenString, err := token.SignedString(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})
		return
	}
	jwtOutput := JWTOutput{
		Token:   tokenString,
		Expires: expirationTime,
	}
	c.JSON(http.StatusOK, jwtOutput)
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

func (handler *AuthHandler) RefreshHandler(c *gin.Context) {
	tokenValue := c.GetHeader("Authorization")
	claims := &Claims{}
	tkn, err := jwt.ParseWithClaims(tokenValue, claims,
		func(token *jwt.Token) (interface{}, error) {
			return key, nil
		})
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.Message{Message: err.Error()})
		return
	}
	if tkn == nil || !tkn.Valid {
		c.JSON(http.StatusUnauthorized, models.Message{Message: "Invalid Token"})
		return
	}
	// if time.Unix(claims.ExpiresAt.Unix(), 0).Sub(time.Now()) > 30*time.Second {
	// 	c.JSON(http.StatusBadRequest, models.Message{Message: "Token is not expired yet"})
	// 	return
	// }
	expirationTime := time.Now().Add(5 * time.Minute)
	claims.ExpiresAt = jwt.At(expirationTime)
	key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenString, err := token.SignedString(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Message{Message: err.Error()})
		return
	}
	jwtOutput := JWTOutput{
		Token:   tokenString,
		Expires: expirationTime,
	}
	c.JSON(http.StatusOK, jwtOutput)
}

func (handler *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenValue := c.GetHeader("Authorization")
		claims := &Claims{}

		tkn, err := jwt.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (interface{}, error) {
			return key, nil
		})
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if !tkn.Valid {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}
