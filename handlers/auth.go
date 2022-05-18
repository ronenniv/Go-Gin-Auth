package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/auth0-community/go-auth0"
	"github.com/dgrijalva/jwt-go/v4"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/ronenniv/webclient/models"
	"github.com/rs/xid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"gopkg.in/square/go-jose.v2"
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

func (h *AuthHandler) SignInHandlerJWT(c *gin.Context) {
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
	// JWT token
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

func (handler *AuthHandler) AuthMiddlewareAuth0() gin.HandlerFunc {
	return func(c *gin.Context) {
		var auth0Domain = "https://" + os.Getenv("AUTH0_DOMAIN") + "/"
		client := auth0.NewJWKClient(auth0.JWKClientOptions{URI: auth0Domain + ".well-known/jwks.json"}, nil)
		configuration := auth0.NewConfiguration(client, []string{os.Getenv("AUTH0_API_IDENTIFIER")}, auth0Domain, jose.RS256)
		validator := auth0.NewValidator(configuration, nil)
		_, err := validator.ValidateRequest(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.Message{Error: "Invalid Token"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (handler *AuthHandler) AuthMiddlewareJWT() gin.HandlerFunc {
	// JWT ssession
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
