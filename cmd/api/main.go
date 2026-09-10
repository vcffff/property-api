package main

import (
	"dev/api-task-manager/configs"
	"dev/api-task-manager/internal/features/auth"
	"dev/api-task-manager/internal/features/property"
	"dev/api-task-manager/internal/features/user"
	"dev/api-task-manager/internal/platform/database"
	"dev/api-task-manager/internal/platform/middleware/limiter"
	"dev/api-task-manager/internal/routes"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	port := ":8080"

	cfg := configs.LoadConfig()

	db, err := database.ConnectDB(cfg)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	if err := db.AutoMigrate(
		&user.User{},
		&property.Property{},
	); err != nil {
		log.Fatal("Migration failed:", err)
	}

	fmt.Println("Migration completed")
	fmt.Println("HOST:", cfg.DBHost)

	authService := auth.NewServiceAuth(db)
	authHandler := auth.NewHandler(authService)

	router := gin.Default()
	router.Use(limiter.RateLimitMiddleware())
	routes.SetUpRoutes(router, authHandler)

	router.Run(port)
}
