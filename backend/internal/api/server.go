package api

import (
	"github.com/gin-gonic/gin"
)

func StartServer(router *gin.Engine, port string) {

	router.Run(":" + port)
}