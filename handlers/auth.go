package handlers

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/ronenniv/Go-Gin-Auth/models"
	"github.com/ronenniv/Go-Gin-Auth/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type AuthHandler struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

// CustomClaims contains custom data we want from the token.
type CustomClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
}

// Validate does nothing for this example, but we need
// it to satisfy validator.CustomClaims interface.
func (c CustomClaims) Validate(ctx context.Context) error {
	return nil
}

type JWTOutput struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

func NewAuthHAndler(collection *mongo.Collection, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		collection: collection,
		logger:     logger,
	}
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

// var (
// 	// The signing key for the token.
// 	signingKey = []byte(os.Getenv("AUTH0_CLIENT_SECRET"))

// 	// The issuer of our token.
// 	// issuer = os.Getenv("AUTH0_DOMAIN")

// 	// The audience of our token.
// 	audience = []string{os.Getenv("AUTH0_AUDIENCE")}

// 	// Our token must be signed using this data.
// 	keyFunc = func(ctx context.Context) (interface{}, error) {
// 		return signingKey, nil
// 	}

// 	// We want this struct to be filled in with
// 	// our custom claims from the token.
// 	customClaims = func() validator.CustomClaims {
// 		return &CustomClaims{}
// 	}
// )

// // CustomClaimsExample contains custom data we want from the token.
// type CustomClaimsExample struct {
// Name         string `json:"name"`
// Username     string `json:"username"`
// ShouldReject bool   `json:"shouldReject,omitempty"`
// }

// // Validate errors out if `ShouldReject` is true.
//
//	func (c *CustomClaimsExample) Validate(ctx context.Context) error {
//		if c.ShouldReject {
//			return errors.New("should reject was set to true")
//		}
//		return nil
//	}
// func (c *CustomClaimsExample) Validate(ctx context.Context) error {
// 	return nil
// }

// checkJWT is a gin.HandlerFunc middleware
// that will check the validity of our JWT.
// func (ah *AuthHandler) CheckJWT() gin.HandlerFunc {
// 	// Set up the validator.
// 	issuer, err := url.Parse("https://" + os.Getenv("AUTH0_DOMAIN") + "/")
// 	if err != nil {
// 		log.Fatal("Failed to parse the issuer url", err)
// 	}
// 	jwtValidator, err := validator.New(
// 		keyFunc,
// 		validator.RS256,
// 		issuer.String(),
// 		audience,
// 		validator.WithCustomClaims(customClaims),
// 		validator.WithAllowedClockSkew(30*time.Second),
// 	)
// 	if err != nil {
// 		ah.logger.Error("failed to set up the validator", zap.Error(err))
// 	}

// 	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
// 		ah.logger.Error("Encountered error while validating JWT", zap.Error(err))
// 	}

// 	middleware := jwtmiddleware.New(
// 		jwtValidator.ValidateToken,
// 		jwtmiddleware.WithErrorHandler(errorHandler),
// 	)

// 	return func(ctx *gin.Context) {
// 		encounteredError := true
// 		var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
// 			encounteredError = false
// 			ctx.Request = r
// 			ctx.Next()
// 		}

// 		middleware.CheckJWT(handler).ServeHTTP(ctx.Writer, ctx.Request)

// 		if encounteredError {
// 			ctx.AbortWithStatusJSON(
// 				http.StatusUnauthorized,
// 				map[string]string{"message": "JWT is invalid."},
// 			)
// 		}
// 	}
// }
