package handlers

import (
	"context"
	"crypto/sha256"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/ronenniv/Go-Gin-Auth/models"
	"github.com/rs/xid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type AuthHandler struct {
	collection *mongo.Collection
	ctx        context.Context
	logger     *zap.Logger
}

func NewAuthHAndler(collection *mongo.Collection, ctx context.Context, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		collection: collection,
		ctx:        ctx,
		logger:     logger,
	}
}

func (h *AuthHandler) SignInHandlerCookie(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})
		return
	}

	sha := sha256.New()
	sha.Write([]byte(user.Password))
	// fetch from mongo
	filter := bson.M{"username": user.Username, "password": sha.Sum(nil)}
	cur := h.collection.FindOne(h.ctx, filter)
	if cur.Err() != nil {
		h.logger.Info("username or password not found", zap.Error(cur.Err()))
		c.JSON(http.StatusUnauthorized, models.Message{Message: "Incorrect user or password"})
		return
	}

	// session cookie
	sessionToken := xid.New().String()
	session := sessions.Default(c)
	session.Set("username", user.Username)
	session.Set("token", sessionToken)
	session.Options(sessions.Options{MaxAge: 10 * 60}) // 10 minutes age
	if err := session.Save(); err != nil {
		h.logger.Info("cannot save session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
		return
	}
	h.logger.Debug("User signed in", zap.String("username", user.Username))
	c.JSON(http.StatusOK, models.User{Username: user.Username})
}

func (h *AuthHandler) AddUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Info("", zap.Error(err))
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
		h.logger.Info("", zap.Error(cur.Err()))
		c.JSON(http.StatusBadRequest, models.Message{Message: "user already exit"})
		return
	} else if cur.Err() != mongo.ErrNoDocuments {
		h.logger.Info("", zap.Error(cur.Err()))
		c.JSON(http.StatusInternalServerError, models.Message{Message: cur.Err().Error()})
		return
	}
	insert := bson.D{
		{"username", user.Username},
		{"password", sha.Sum(nil)}}
	_, err := h.collection.InsertOne(h.ctx, insert)
	if err != nil {
		h.logger.Info("", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})
		return
	}
	h.logger.Info("user added", zap.String("", user.Username))
	c.JSON(http.StatusOK, user.Username)
}

func (h *AuthHandler) AuthMiddlewareCookie() gin.HandlerFunc {
	// Cookie session
	return func(c *gin.Context) {
		session := sessions.Default(c)
		sessionToken := session.Get("token")
		h.logger.Debug("cookie username", zap.Any("", session.Get("username")))
		if sessionToken == nil {
			h.logger.Info("user not logged in", zap.Any("", session.Get("username")))
			c.JSON(http.StatusForbidden, models.Message{Error: "Not logged in"})
			c.Abort()
		}
		c.Next()
	}
}

func (h *AuthHandler) SignOutHandlerCookie(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		h.logger.Info("cannot save session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.Message{Error: err.Error()})
		return
	}
	h.logger.Info("user logged out", zap.Any("", session.Get("username")))
	c.JSON(http.StatusOK, models.Message{Message: "Signed out"})
}
