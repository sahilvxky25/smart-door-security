package repository

import (
	"errors"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"gorm.io/gorm"
)

type FamilyRepo struct {
	db *gorm.DB
}

func NewFamilyRepo(db *gorm.DB) *FamilyRepo {
	return &FamilyRepo{db: db}
}

func (r *FamilyRepo) Create(member *models.FamilyMember) error {
	return r.db.Create(member).Error
}

func (r *FamilyRepo) List(userID uint) ([]models.FamilyMember, error) {
	var members []models.FamilyMember
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *FamilyRepo) Get(userID uint, id uint) (*models.FamilyMember, error) {
	var m models.FamilyMember
	if err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *FamilyRepo) GetByName(userID uint, name string) (*models.FamilyMember, error) {
	var m models.FamilyMember
	if err := r.db.Where("user_id = ? AND name = ?", userID, name).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *FamilyRepo) Update(member *models.FamilyMember) error {
	return r.db.Save(member).Error
}

func (r *FamilyRepo) Delete(userID uint, id uint) error {
	return r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&models.FamilyMember{}).Error
}
