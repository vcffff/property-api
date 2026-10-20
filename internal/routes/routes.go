package routes

import (
	"dev/api-task-manager/internal/features/application"
	"dev/api-task-manager/internal/features/auth"
	"dev/api-task-manager/internal/features/property"
	"dev/api-task-manager/internal/features/viewing"
	"dev/api-task-manager/internal/platform/middleware"
	"os"

	"github.com/gin-gonic/gin"
)

func SetUpRoutes(router *gin.Engine, authHandler *auth.Handler, propertyHandler *property.PropertyHandler, applicationHandler *application.ApplicationHandler, viewingHandler *viewing.ViewingHandler) {
	api := router.Group("/api")
	{
		v1 := api.Group("/v1")
		{

			v1.POST("/register", authHandler.Register)
			v1.POST("/login", authHandler.Login)
			v1.POST("/auth/refresh", authHandler.RefreshToken)
			v1.POST("/auth/logout", authHandler.Logout)

			protected := v1.Group("/")
			protected.Use(middleware.AuthMiddleware())

			{
				// Never expose self-service privilege changes by default or in production.
				if os.Getenv("APP_ENV") == "development" && os.Getenv("ENABLE_DEV_ROLE_ENDPOINT") == "true" {
					protected.PATCH("/dev/users/me/role", authHandler.UpdateDevRole)
				}
				protected.GET("/users/me", authHandler.Me)
				protected.GET("/properties", propertyHandler.GetAll)
				protected.GET("/properties/:id", propertyHandler.GetByID)
				protected.GET("/properties/:id/available-slots", viewingHandler.GetAvailableSlots)
				protected.POST("/applications", applicationHandler.Create)
				protected.GET("/applications/me", applicationHandler.GetMine)
				protected.POST("/viewings", viewingHandler.Book)
				protected.GET("/viewings/me", viewingHandler.GetMine)
				admin := protected.Group("/admin")
				admin.Use(middleware.RoleMiddleware("admin"))
				{
					admin.GET("/applications", applicationHandler.GetAll)
					admin.PATCH("/applications/:id/status", applicationHandler.UpdateStatus)
					admin.GET("/viewings", viewingHandler.GetAll)
					admin.PATCH("/viewings/:id/status", viewingHandler.UpdateStatus)
				}

				// Admin-only property endpoints keep their public API paths.
				protected.POST("/properties", middleware.RoleMiddleware("admin"), propertyHandler.Create)
				protected.PATCH("/properties/:id", middleware.RoleMiddleware("admin"), propertyHandler.Update)
				protected.DELETE("/properties/:id", middleware.RoleMiddleware("admin"), propertyHandler.Delete)
			}
		}
	}
}
