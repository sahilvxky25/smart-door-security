package models

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `json:"name"`
	Email        string    `gorm:"unique" json:"email"`
	PhotoURL     string    `json:"photo_url"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}