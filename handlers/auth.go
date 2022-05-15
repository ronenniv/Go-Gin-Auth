package handlers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go/v4"
	"github.com/gin-gonic/gin"
	"github.com/ronenniv/webclient/models"
)

type AuthHandler struct{}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

type JWTOutput struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

var key *ecdsa.PrivateKey

func (h *AuthHandler) SignInHandler(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, models.Message{Message: err.Error()})
		return
	}
	if user.Username != "admin" || user.Password != "password" {
		c.JSON(http.StatusUnauthorized, models.Message{Message: "Invalid username or password"})
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
