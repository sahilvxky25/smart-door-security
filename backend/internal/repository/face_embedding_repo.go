package repository

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
	"gorm.io/gorm"
)

type FaceEmbeddingRepo struct {
	db *gorm.DB
}

func NewFaceEmbeddingRepo(db *gorm.DB) *FaceEmbeddingRepo {
	return &FaceEmbeddingRepo{db: db}
}

func (r *FaceEmbeddingRepo) Upsert(userID uint, memberID uint, name string, imageURL string, embedding []float64) error {
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("marshal embedding: %w", err)
	}

	var existing models.FaceEmbedding
	err = r.db.Where("user_id = ? AND family_member_id = ?", userID, memberID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.Create(&models.FaceEmbedding{
			UserID:         userID,
			FamilyMemberID: memberID,
			Name:           name,
			ImageURL:       imageURL,
			EmbeddingJSON:  string(embeddingJSON),
		}).Error
	}
	if err != nil {
		return err
	}

	existing.Name = name
	existing.ImageURL = imageURL
	existing.EmbeddingJSON = string(embeddingJSON)
	return r.db.Save(&existing).Error
}

func (r *FaceEmbeddingRepo) Delete(userID uint, memberID uint) error {
	return r.db.Where("user_id = ? AND family_member_id = ?", userID, memberID).Delete(&models.FaceEmbedding{}).Error
}

func (r *FaceEmbeddingRepo) ListCandidates(userID uint) ([]services.FaceCandidate, error) {
	type candidateRow struct {
		FamilyMemberID uint
		Name           string
		EmbeddingJSON  string
	}

	var rows []candidateRow
	err := r.db.
		Table("face_embeddings").
		Select("face_embeddings.family_member_id, family_members.name, face_embeddings.embedding_json").
		Joins("JOIN family_members ON family_members.id = face_embeddings.family_member_id AND family_members.user_id = face_embeddings.user_id").
		Where("face_embeddings.user_id = ?", userID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	candidates := make([]services.FaceCandidate, 0, len(rows))
	for _, row := range rows {
		var embedding []float64
		if err := json.Unmarshal([]byte(row.EmbeddingJSON), &embedding); err != nil {
			return nil, fmt.Errorf("decode embedding for member %d: %w", row.FamilyMemberID, err)
		}
		candidates = append(candidates, services.FaceCandidate{
			MemberID:  row.FamilyMemberID,
			Name:      row.Name,
			Embedding: embedding,
		})
	}
	return candidates, nil
}
