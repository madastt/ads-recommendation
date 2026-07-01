package models

import (
	"time"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Campaign struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Budget    float64   `json:"budget"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	CreatedAt time.Time `json:"created_at"`
}

type Ad struct {
	ID              string    `json:"id"`
	CampaignID      string    `json:"campaign_id"`
	ImageURL        string    `json:"image_url"`
	ContextFeatures string    `json:"context_features"`
	CreatedAt       time.Time `json:"created_at"`
}

type Event struct {
	ID          string    `json:"id"`
	AdID        string    `json:"ad_id"`
	EventType   string    `json:"event_type"`
	UserContext string    `json:"user_context"`
	CreatedAt   time.Time `json:"created_at"`
}
