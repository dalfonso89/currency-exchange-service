package models

import "time"

type RatesResponse struct {
	Base      string             `json:"base"`
	Timestamp int64              `json:"timestamp"`
	Rates     map[string]float64 `json:"rates"`
	Provider  string             `json:"provider"`
}

type ConvertQuery struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
}

type ConvertResponse struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	Amount    float64 `json:"amount"`
	Rate      float64 `json:"rate"`
	Converted float64 `json:"converted"`
	Provider  string  `json:"provider"`
}

type CacheEntry struct {
	Data      RatesResponse
	ExpiresAt time.Time
}

type HealthCheck struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
}

type APIResponse struct {
	Data   interface{} `json:"data"`
	Status int         `json:"status"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
