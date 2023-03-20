package handlers

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/ronenniv/Go-Gin-Auth/models"
	"github.com/ronenniv/Go-Gin-Auth/types"
	"github.com/rs/xid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type AuthHandler struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

func NewAuthHAndler(collection *mongo.Collection, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		collection: collection,
		logger:     logger,
	}
}

func (h *AuthHandler) SignInHandlerCookie(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Error: err.Error()})

		return
	}
	// encrypt password
	sha := sha256.New()
	sha.Write([]byte(user.Password))
	// fetch from mongo the user and password
	filter := bson.M{"username": user.Username, "password": sha.Sum(nil)}

	ctx, cancel := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel()

	cur := h.collection.FindOne(ctx, filter)
	if cur.Err() != nil {
		h.logger.Info("username or password not found", zap.Error(cur.Err()))
		c.JSON(http.StatusUnauthorized, models.Message{Message: "Incorrect user or password"})

		return
	}
	// session cookie
	const maxAgeSeconds = 10 * 60 // 10 minutes age

	sessionToken := xid.New().String()
	session := sessions.Default(c)
	session.Set("username", user.Username)
	session.Set("token", sessionToken)
	session.Options(sessions.Options{MaxAge: maxAgeSeconds})

	if err := session.Save(); err != nil {
		h.logger.Info("cannot save session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})

		return
	}

	h.logger.Debug("User signed in", zap.String("username", user.Username))
	c.JSON(http.StatusOK, models.User{Username: user.Username})
}

// AddUser create a new user.
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

	ctx1, cancel1 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel1()

	cur := h.collection.FindOne(ctx1, filter)
	if cur.Err() == nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: "user already exit"})

		return
	} else if !errors.Is(cur.Err(), mongo.ErrNoDocuments) {
		c.JSON(http.StatusInternalServerError, models.Message{Message: cur.Err().Error()})

		return
	}

	insert := bson.D{
		{"username", user.Username},
		{"password", sha.Sum(nil)}}

	ctx2, cancel2 := context.WithTimeout(context.Background(), types.MongoCtxTimeout)
	defer cancel2()

	_, err := h.collection.InsertOne(ctx2, insert)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})

		return
	}

	c.JSON(http.StatusOK, user.Username)
}

func (h *AuthHandler) AuthMiddlewareCookie() gin.HandlerFunc {
	// Cookie session
	return func(c *gin.Context) {
		session := sessions.Default(c)
		h.logger.Debug("cookie", zap.Any("username", session.Get("username")))

		sessionToken := session.Get("token")
		if sessionToken == nil {
			h.logger.Info("user not logged in", zap.Any("username", session.Get("username")))
			c.JSON(http.StatusForbidden, models.Message{Error: "User not logged in"})
			c.Abort()
		}

		c.Next()
	}
}

func (h *AuthHandler) LogoutHandlerCookie(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()

	if err := session.Save(); err != nil {
		h.logger.Info("cannot save session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})

		return
	}

	h.logger.Info("user logged out", zap.Any("username", session.Get("username")))
	c.JSON(http.StatusOK, models.Message{Message: "signed out"})
}
