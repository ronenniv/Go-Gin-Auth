package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ronenniv/go-gin-auth/models"
	"github.com/stretchr/testify/assert"
)

var username string

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

var jsonUserBody string
var router *gin.Engine

func TestMain(m *testing.M) {
	log.Println("testing main setup")

	// username = RandStringBytes(8)
	// jsonUserBody = fmt.Sprintf("{\"username\": \"%s\", \"password\" : \"password\"}", username)

	router = setupRouter()

	code := m.Run()
	os.Exit(code)
}

func TestAddUser(t *testing.T) {
	username = RandStringBytes(8)
	jsonUserBody = fmt.Sprintf("{\"username\": \"%s\", \"password\" : \"password\"}", username)
	w := httptest.NewRecorder()

	req, _ := http.NewRequest("POST", "/adduser", bytes.NewBufferString(jsonUserBody))
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	expectedBody, _ := json.Marshal(models.Message{Message: username})
	assert.Equal(t, string(expectedBody), w.Body.String())
}

func TestLogin(t *testing.T) {
	// w := httptest.NewRecorder()
	// req, _ := http.NewRequest("POST", "/adduser", bytes.NewBufferString(jsonUserBody))
	// router.ServeHTTP(w, req)

	// assert.Equal(t, 200, w.Code)
	TestAddUser(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(jsonUserBody))
	req.Header.Add("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	expectedBody, _ := json.Marshal(models.User{Username: username})
	assert.Equal(t, string(expectedBody), w.Body.String())
}
