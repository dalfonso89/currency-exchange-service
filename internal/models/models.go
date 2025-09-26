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

