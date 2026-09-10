package routes

import (
	"dev/api-task-manager/internal/features/auth"

	"dev/api-task-manager/internal/platform/middleware"

	"github.com/gin-gonic/gin"
)

func SetUpRoutes(router *gin.Engine, authHandler *auth.Handler) {
	api := router.Group("/api")
	{
		v1 := api.Group("/v1")
		{

			v1.POST("/register", authHandler.Register)
			v1.POST("/login", authHandler.Login)

			protected := v1.Group("/")
			protected.Use(middleware.AuthMiddleware())
			{
				protected.GET("/users/me", authHandler.Me)
			}
		}
	}
}
