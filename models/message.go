package models

type Message struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
