package api

import (
	"fmt"
	"net/http"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/controllers"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/repository"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/storage"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/webrtc"

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
	eventService *services.EventService,
	signalingHub *webrtc.Hub,
	mediaStore *storage.MediaStorage,
) *gin.Engine {

	r := gin.Default()

	// Swagger docs
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/health", healthCheck)

	// Authentication
	r.POST("/auth/signup", signUpHandler(auth))
	r.POST("/auth/signin", signInHandler(auth))

	r.POST("/door/unlock", unlockDoorManual(door, eventService))
	r.POST("/door/lock", lockDoor(door))

	r.GET("/users", showUsersHandler(db))
	r.POST("/users", createUserHandler(db))

	r.GET("/events", listEventsHandler(eventService))
	r.GET("/events/:id", getEventHandler(eventService))

	// Family member management + face enrollment
	familyRepo := repository.NewFamilyRepo(db)
	fc := controllers.NewFamilyController(familyRepo, face, mediaStore)
	r.GET("/family", fc.ListMembers)
	r.POST("/family", fc.CreateMember)
	r.GET("/family/:id", fc.GetMember)
	r.PUT("/family/:id", fc.UpdateMember)
	r.DELETE("/family/:id", fc.DeleteMember)
	r.POST("/family/:id/enroll", fc.EnrollFace)
	r.DELETE("/family/:id/enroll", fc.UnenrollFace)

	// Image proxy — fetches objects from MinIO and streams them to clients
	r.GET("/images/*path", imageProxyHandler(mediaStore))

	// WebRTC signaling WebSocket
	r.GET("/ws/signaling", signalingHub.HandleWebSocket)

	// Static web files (door.html, owner.html)
	r.Static("/web", "./web")

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

// unlockDoorManual godoc
// @Summary Unlock door (manual)
// @Description Sends an MQTT command to unlock the door servo and logs a MANUAL_UNLOCK event
// @Tags door
// @Produce json
// @Success 200 {object} map[string]string
// @Router /door/unlock [post]
func unlockDoorManual(door *services.DoorService, events *services.EventService) gin.HandlerFunc {
	return func(c *gin.Context) {
		door.UnlockDoor()
		events.LogEvent(models.EventManualUnlock, nil, "")
		c.JSON(200, gin.H{"status": "door opened"})
	}
}

// lockDoor godoc
// @Summary Lock door
// @Description Sends an MQTT command to lock the door servo
// @Tags door
// @Produce json
// @Success 200 {object} map[string]string
// @Router /door/lock [post]
func lockDoor(door *services.DoorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		door.LockDoor()
		c.JSON(200, gin.H{"status": "door locked"})
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

// listEventsHandler godoc
// @Summary List events
// @Description Returns the most recent events
// @Tags events
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /events [get]
func listEventsHandler(events *services.EventService) gin.HandlerFunc {
	return func(c *gin.Context) {
		eventList, err := events.ListEvents(100, 0)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to fetch events"})
			return
		}
		c.JSON(200, gin.H{"events": eventList})
	}
}

// getEventHandler godoc
// @Summary Get event by ID
// @Description Returns a single event
// @Tags events
// @Produce json
// @Param id path int true "Event ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /events/{id} [get]
func getEventHandler(events *services.EventService) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		var id uint
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			c.JSON(400, gin.H{"error": "invalid event id"})
			return
		}
		event, err := events.GetEvent(id)
		if err != nil {
			c.JSON(404, gin.H{"error": "event not found"})
			return
		}
		c.JSON(200, gin.H{"event": event})
	}
}

// SignUpRequest is the request body for POST /auth/signup
type SignUpRequest struct {
	Name     string `json:"name"  binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// signUpHandler godoc
// @Summary Register a new user
// @Description Creates a new user account and returns a JWT
// @Tags auth
// @Accept json
// @Produce json
// @Param body body SignUpRequest true "Sign-up credentials"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /auth/signup [post]
func signUpHandler(auth *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SignUpRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		token, user, err := auth.SignUp(req.Name, req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"token": token, "user": user})
	}
}

// SignInRequest is the request body for POST /auth/signin
type SignInRequest struct {
	Name     string `json:"name"     binding:"required"`
	Password string `json:"password" binding:"required"`
}

// signInHandler godoc
// @Summary Sign in
// @Description Authenticates by name + password and returns a JWT
// @Tags auth
// @Accept json
// @Produce json
// @Param body body SignInRequest true "Sign-in credentials"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /auth/signin [post]
func signInHandler(auth *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SignInRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		token, user, err := auth.SignIn(req.Name, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
	}
}

func imageProxyHandler(media *storage.MediaStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		// path param includes leading slash, e.g. "/snapshot_123.jpg"
		objectName := c.Param("path")
		if len(objectName) > 0 && objectName[0] == '/' {
			objectName = objectName[1:]
		}
		if objectName == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		obj, contentType, err := media.GetObject(c.Request.Context(), objectName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
			return
		}
		defer obj.Close()

		c.DataFromReader(http.StatusOK, -1, contentType, obj, nil)
	}
}