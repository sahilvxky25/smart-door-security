package services

import (
	"errors"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db     *gorm.DB
	secret string
}

func NewAuthService(db *gorm.DB, secret string) *AuthService {
	return &AuthService{db: db, secret: secret}
}

// SignUp creates a new user and returns a signed JWT.
func (a *AuthService) SignUp(name, email, password string) (string, *models.User, error) {
	var existing models.User
	if err := a.db.Where("name = ? OR email = ?", name, email).First(&existing).Error; err == nil {
		return "", nil, errors.New("name or email already taken")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}

	user := models.User{Name: name, Email: email, PasswordHash: string(hash)}
	if err := a.db.Create(&user).Error; err != nil {
		return "", nil, err
	}

	token, err := utils.GenerateToken(user.ID, user.Name, a.secret)
	if err != nil {
		return "", nil, err
	}
	return token, &user, nil
}

// SignIn verifies name + password and returns a signed JWT.
func (a *AuthService) SignIn(name, password string) (string, *models.User, error) {
	var user models.User
	if err := a.db.Where("name = ?", name).First(&user).Error; err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	token, err := utils.GenerateToken(user.ID, user.Name, a.secret)
	if err != nil {
		return "", nil, err
	}
	return token, &user, nil
}
