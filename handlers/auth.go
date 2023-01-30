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

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/ronenniv/Go-Gin-Auth/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type AuthHandler struct {
	collection *mongo.Collection
	ctx        context.Context
	logger     *zap.Logger
}

type UserClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
}

type JWTOutput struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

var key *ecdsa.PrivateKey

func init() {
	// we want to create the key only once for all users
	var err error
	key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
}

func NewAuthHAndler(collection *mongo.Collection, ctx context.Context, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		collection: collection,
		ctx:        ctx,
		logger:     logger,
	}
}

func (h *AuthHandler) SignInHandlerJWT(c *gin.Context) {
	// JWT session - create new session
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Error: err.Error()})
		return
	}

	sha := sha256.New()
	sha.Write([]byte(user.Password))
	// check if user and password existing in DB
	filter := bson.M{"username": user.Username, "password": sha.Sum(nil)}
	cur := h.collection.FindOne(h.ctx, filter)
	if cur.Err() != nil {
		h.logger.Info("username or password not found", zap.String("username", user.Username), zap.Error(cur.Err()))
		c.JSON(http.StatusUnauthorized, models.Message{Error: "Incorrect user or password"})
		return
	}
	// JWT token
	expirationTime := time.Now().Add(10 * time.Minute)
	claims := &UserClaims{
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	// get the string token so can return it in body
	tokenString, err := token.SignedString(key)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
		return
	}
	jwtOutput := JWTOutput{
		Token:   tokenString,
		Expires: expirationTime,
	}
	h.logger.Info("user login", zap.String("username", claims.Username))
	c.JSON(http.StatusOK, jwtOutput)
}

func (h *AuthHandler) AddUser(c *gin.Context) {
	// add user to mongodb
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Error: err.Error()})
		return
	}
	sha := sha256.New()
	sha.Write([]byte(user.Password))
	user.Password = string(sha.Sum(nil))
	// chec if user already exist
	filter := bson.D{
		{"username", user.Username}}
	cur := h.collection.FindOne(h.ctx, filter)
	if cur.Err() == nil {
		h.logger.Info("user already exit", zap.Error(cur.Err()))
		c.JSON(http.StatusBadRequest, models.Message{Error: "user already exit"})
		return
	} else if cur.Err() != mongo.ErrNoDocuments {
		h.logger.Info("", zap.Error(cur.Err()))
		c.JSON(http.StatusInternalServerError, models.Message{Error: cur.Err().Error()})
		return
	}
	insert := bson.D{
		{"username", user.Username},
		{"password", sha.Sum(nil)}}
	_, err := h.collection.InsertOne(h.ctx, insert)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.User{Username: user.Username})
}

func (h *AuthHandler) RefreshHandler(c *gin.Context) {
	// JWT session - refresh/renew session
	tokenValue := c.GetHeader("Authorization")
	userClaims := &UserClaims{}
	token, err := jwt.ParseWithClaims(tokenValue, userClaims,
		func(token *jwt.Token) (interface{}, error) {
			return key.Public(), nil
		})
	if err != nil || token == nil || !token.Valid {
		h.logger.Error("invalid token", zap.String("username", userClaims.Username), zap.Error(err))
		c.JSON(http.StatusUnauthorized, models.Message{Error: "Invalid Token"})
		return
	}
	expirationTime := time.Now().Add(5 * time.Minute)
	userClaims.ExpiresAt = jwt.NewNumericDate(expirationTime)
	token = jwt.NewWithClaims(jwt.SigningMethodES256, userClaims)
	// get the string token so can return it in body
	tokenString, err := token.SignedString(key)
	if err != nil {
		h.logger.Error("", zap.String("username", userClaims.Username), zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
		return
	}
	jwtOutput := JWTOutput{
		Token:   tokenString,
		Expires: expirationTime,
	}
	c.JSON(http.StatusOK, jwtOutput)
}

func (h *AuthHandler) AuthMiddlewareJWT() gin.HandlerFunc {
	// JWT ssession
	return func(c *gin.Context) {
		tokenValue := c.GetHeader("Authorization")
		userClaims := &UserClaims{}
		token, err := jwt.ParseWithClaims(tokenValue, userClaims, func(token *jwt.Token) (interface{}, error) {
			return key.Public(), nil
		})
		if err != nil || token == nil || !token.Valid {
			h.logger.Error("invalid token", zap.String("username", userClaims.Username), zap.Error(err))
			c.JSON(http.StatusUnauthorized, models.Message{Error: "Invalid Token"})
			c.Abort()
			return
		}
		h.logger.Debug("token validated", zap.String("username", userClaims.Username))
		c.Next()
	}
}

func (h *AuthHandler) LogoutHandlerJWT(c *gin.Context) {
	// The idea with JWT that it's stateless session so the session is not stored
	// and as such there is no logout. The client should refresh the session to keep the active session
	// one way to logout is to change the expiration time and return new session to the client
	// JWT session - refresh session with expired time
	tokenValue := c.GetHeader("Authorization")
	userClaims := &UserClaims{}
	token, err := jwt.ParseWithClaims(tokenValue, userClaims,
		func(token *jwt.Token) (interface{}, error) {
			return key.Public(), nil
		})
	if err != nil || token == nil || !token.Valid {
		h.logger.Error("invalid token", zap.String("username", userClaims.Username), zap.Error(err))
		c.JSON(http.StatusUnauthorized, models.Message{Error: "Invalid Token"})
		return
	}
	expirationTime := time.Now() // cuurent time so next use with the token it'll be expired
	userClaims.ExpiresAt = jwt.NewNumericDate(expirationTime)
	token = jwt.NewWithClaims(jwt.SigningMethodES256, userClaims)
	// get the string token so can return it in body
	tokenString, err := token.SignedString(key)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
		return
	}
	jwtOutput := JWTOutput{
		Token:   tokenString,
		Expires: expirationTime,
	}
	h.logger.Debug("update token with expiration to now", zap.String("username", userClaims.Username))
	c.JSON(http.StatusOK, jwtOutput)
}

func (h *AuthHandler) CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}
