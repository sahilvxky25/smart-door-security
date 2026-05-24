package models

import "time"

type FaceEmbedding struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;uniqueIndex:idx_face_embedding_member" json:"user_id"`
	FamilyMemberID uint      `gorm:"not null;uniqueIndex:idx_face_embedding_member" json:"family_member_id"`
	Name           string    `gorm:"not null" json:"name"`
	ImageURL       string    `json:"image_url"`
	EmbeddingJSON  string    `gorm:"type:text;not null" json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
