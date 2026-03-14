package api

import (
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"

	"github.com/gin-gonic/gin"
	_ "github.com/gottatouchsomegrass/smart-door-backend/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func NewRouter(
	db *gorm.DB,
	auth *services.AuthService,
	door *services.DoorService,
	camera *services.CameraService,
	intrusion *services.IntrusionService,
	notify *services.NotificationService,
	face *services.FaceService,
) *gin.Engine {

	r := gin.Default()

	// Swagger docs
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/health", healthCheck)
	r.POST("/door/unlock", unlockDoor(door))

	r.GET("/users", showUsersHandler(db))
	r.POST("/users", createUserHandler(db))

	return r
}

// healthCheck godoc
// @Summary Health check
// @Description Returns the health status of the API
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// unlockDoor godoc
// @Summary Unlock door
// @Description Sends an MQTT command to unlock the door servo
// @Tags door
// @Produce json
// @Success 200 {object} map[string]string
// @Router /door/unlock [post]
func unlockDoor(door *services.DoorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		door.UnlockDoor()
		c.JSON(200, gin.H{"status": "door opened"})
	}
}

// showUsersHandler godoc
// @Summary List all users
// @Description Returns a list of all registered users
// @Tags users
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /users [get]
func showUsersHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var users []models.User
		if result := db.Find(&users); result.Error != nil {
			c.JSON(500, gin.H{"error": "failed to fetch users"})
			return
		}
		c.JSON(200, gin.H{"users": users})
	}
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// createUserHandler godoc
// @Summary Create a new user
// @Description Creates a new user with name, email, and password
// @Tags users
// @Accept json
// @Produce json
// @Param user body CreateUserRequest true "User to create"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users [post]
func createUserHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		var req CreateUserRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request"})
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to hash password"})
			return
		}

		user := models.User{
			Name:         req.Name,
			Email:        req.Email,
			PasswordHash: string(hashedPassword),
		}

		result := db.Create(&user)

		if result.Error != nil {
			c.JSON(500, gin.H{"error": "failed to create user"})
			return
		}

		c.JSON(201, gin.H{
			"message": "user created",
			"user":    user,
		})
	}
}