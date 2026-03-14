package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/config"
)

type Server struct {
	httpServer *http.Server
}

func StartServer(router *gin.Engine, cfg *config.Config) *Server {
	srv := &http.Server{
		Addr:    ":" + cfg.PORT,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("HTTP server failed: " + err.Error())
		}
	}()

	return &Server{httpServer: srv}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}