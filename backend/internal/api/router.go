package api

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gottatouchsomegrass/smart-door-backend/internal/api/middleware"
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

// DoorStateResponse is the response body for GET /door/state
type DoorStateResponse struct {
	ExpectedAngle int    `json:"expected_angle" example:"0"`
	State         string `json:"state"          example:"LOCKED"`
}

func NewRouter(
	db *gorm.DB,
	auth *services.AuthService,
	door *services.DoorService,
	securityState *services.SecurityStateService,
	notify *services.NotificationService,
	face *services.FaceService,
	eventService *services.EventService,
	signalingHub *webrtc.Hub,
	mediaStore *storage.MediaStorage,
	jwtSecret string,
) *gin.Engine {

	r := gin.Default()

	// Swagger docs
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/health", healthCheck)

	// Authentication
	r.POST("/auth/signup", signUpHandler(auth))
	r.POST("/auth/signin", signInHandler(auth))
	r.POST("/auth/signout", signOutHandler())

	// Protected Routes
	authGroup := r.Group("/")
	authGroup.Use(middleware.RequireAuth(jwtSecret))

	// Door control
	authGroup.POST("/door/unlock", unlockDoorManual(door, eventService, securityState))
	authGroup.POST("/door/lock", lockDoor(door, eventService, securityState))
	authGroup.GET("/door/state", getDoorState(door))
	authGroup.POST("/security/clear-intrusion", clearIntrusion(securityState, eventService))

	authGroup.GET("/users", showUsersHandler(db))
	authGroup.POST("/users", createUserHandler(db))
	authGroup.POST("/users/fcm-token", updateFCMTokenHandler(db))

	authGroup.GET("/events", listEventsHandler(eventService))
	authGroup.GET("/events/:id", getEventHandler(eventService))

	// Family member management + face enrollment
	familyRepo := repository.NewFamilyRepo(db)
	faceEmbeddingRepo := repository.NewFaceEmbeddingRepo(db)
	fc := controllers.NewFamilyController(familyRepo, faceEmbeddingRepo, face, mediaStore)
	authGroup.GET("/family", fc.ListMembers)
	authGroup.POST("/family", fc.CreateMember)
	authGroup.GET("/family/:id", fc.GetMember)
	authGroup.PUT("/family/:id", fc.UpdateMember)
	authGroup.DELETE("/family/:id", fc.DeleteMember)
	authGroup.POST("/family/:id/enroll", fc.EnrollFace)
	authGroup.DELETE("/family/:id/enroll", fc.UnenrollFace)

	// Profile photo upload
	authGroup.POST("/profile/photo", uploadProfilePhotoHandler(db, mediaStore, face))

	// WebRTC signaling WebSocket
	r.GET("/ws/signaling", signalingHub.HandleWebSocket)

	return r
}

// uploadProfilePhotoHandler handles POST /profile/photo (multipart).
// Saves the uploaded image to Cloudinary and stores the URL in the user record.
func uploadProfilePhotoHandler(db *gorm.DB, store *storage.MediaStorage, faceService *services.FaceService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user name
		name := c.Query("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name query param required"})
			return
		}

		file, header, err := c.Request.FormFile("photo")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "photo field required"})
			return
		}
		defer file.Close()

		// Read & upload to Cloudinary
		data, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
			return
		}

		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "image/jpeg"
		}
		objectName := fmt.Sprintf("profiles/%s_%s", name, header.Filename)
		url, err := store.UploadImage(c.Request.Context(), objectName, data, contentType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
			return
		}

		var user models.User
		if err := db.Where("name = ?", name).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		// Enroll the profile face under the user's own scoped bucket. Family
		// members use their family_member ID; profile photos use the user ID.
		if err := faceService.EnrollFace(user.ID, user.ID, user.Name, data); err != nil {
			// Enrollment failed — clean up the uploaded photo
			_ = store.DeleteObject(c.Request.Context(), objectName)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}

		user.PhotoURL = url
		db.Save(&user)

		c.JSON(http.StatusOK, gin.H{
			"photo_url": url,
			"user":      user,
		})
	}
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
// @Description Sends an MQTT UNLOCK command to the servo and auto-locks after 15 s. Logs a MANUAL_UNLOCK event.
// @Tags door
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /door/unlock [post]
func unlockDoorManual(door *services.DoorService, events *services.EventService, securityState *services.SecurityStateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if securityState != nil {
			if securityState.ClearIntrusion("manual owner unlock") {
				events.LogEvent(models.EventIntrusionCleared, "")
			}
		}
		door.UnlockDoor()
		events.LogEvent(models.EventManualUnlock, "")
		c.JSON(200, gin.H{"status": "door unlocked"})
	}
}

// lockDoor godoc
// @Summary Lock door
// @Description Sends an MQTT LOCK command to the servo. Logs a MANUAL_LOCK event.
// @Tags door
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /door/lock [post]
func lockDoor(door *services.DoorService, events *services.EventService, securityState *services.SecurityStateService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if securityState != nil {
			if securityState.ClearIntrusion("manual owner lock") {
				events.LogEvent(models.EventIntrusionCleared, "")
			}
		}
		door.LockDoor()
		events.LogEvent(models.EventManualLock, "")
		c.JSON(200, gin.H{"status": "door locked"})
	}
}

// clearIntrusion godoc
// @Summary Clear intrusion state
// @Description Clears the active intrusion lockout so visitor authentication can resume.
// @Tags security
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /security/clear-intrusion [post]
func clearIntrusion(securityState *services.SecurityStateService, events *services.EventService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if securityState != nil {
			if securityState.ClearIntrusion("manual owner clear") {
				events.LogEvent(models.EventIntrusionCleared, "")
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "intrusion cleared"})
	}
}

// getDoorState godoc
// @Summary Get current door lock state
// @Description Returns the servo angle and lock state last commanded by the backend (LOCKED / UNLOCKED).
// @Tags door
// @Produce json
// @Security BearerAuth
// @Success 200 {object} DoorStateResponse
// @Failure 401 {object} map[string]string
// @Router /door/state [get]
func getDoorState(door *services.DoorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		angle := door.ExpectedAngle()
		state := "LOCKED"
		if angle != 0 {
			state = "UNLOCKED"
		}
		c.JSON(200, DoorStateResponse{
			ExpectedAngle: angle,
			State:         state,
		})
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
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
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

// UpdateFCMTokenRequest represents the request body for updating the FCM token
type UpdateFCMTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// updateFCMTokenHandler godoc
// @Summary Update FCM Token
// @Description Updates the Firebase Cloud Messaging token for the authenticated user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpdateFCMTokenRequest true "FCM Token"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/fcm-token [post]
func updateFCMTokenHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("user_id").(uint)

		var req UpdateFCMTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
			return
		}

		if err := db.Model(&models.User{}).Where("id = ?", userID).Update("fcm_token", req.Token).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save fcm token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "fcm token updated successfully"})
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
		userID := c.MustGet("user_id").(uint)
		eventList, err := events.ListEvents(userID, 100, 0)
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

// signOutHandler godoc
// @Summary Sign out
// @Description Invalidates the client session. Because JWTs are stateless, the
// client must delete the stored token. The server acknowledges with 200 OK.
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Router /auth/signout [post]
func signOutHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "signed out"})
	}
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
