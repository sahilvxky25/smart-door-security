package api

import (
	"smart-door-backend/internal/services"

	"github.com/gin-gonic/gin"
)

func NewRouter(
	auth *services.AuthService,
	door *services.DoorService,
	camera *services.CameraService,
	intrusion *services.IntrusionService,
	notify *services.NotificationService,
) *gin.Engine {

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.POST("/door/unlock", func(c *gin.Context) {
		door.UnlockDoor()
		c.JSON(200, gin.H{"status": "door opened"})
	})

	return r
}