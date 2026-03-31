package models

import "time"

type FamilyMember struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null" json:"user_id"`
	Name         string    `gorm:"uniqueIndex:idx_family_member_user_name;not null" json:"name"`
	PhotoURL     string    `json:"photo_url"`
	FaceEnrolled bool      `gorm:"default:false" json:"face_enrolled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
