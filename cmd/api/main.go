package main

import (
	"dev/api-task-manager/configs"
	"dev/api-task-manager/internal/features/auth"
	"dev/api-task-manager/internal/features/property"
	"dev/api-task-manager/internal/features/user"
	"dev/api-task-manager/internal/infrastructure/redis"
	"dev/api-task-manager/internal/platform/database"
	"dev/api-task-manager/internal/platform/middleware/limiter"
	"dev/api-task-manager/internal/routes"
	"dev/api-task-manager/internal/session"
	"fmt"
	"log"
	"time"

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

	redisClient, err := redis.ConnectRedis(cfg.RedisHost)

	if err != nil {
		log.Fatal("Redis connection failed:", err)
	}
	defer redisClient.Close()
	fmt.Println("Redis connected")

	sessionService := session.NewService(redisClient)
	authService := auth.NewServiceAuth(db, sessionService)
	authHandler := auth.NewHandler(authService)

	propertyRepository := property.NewPropertyRepository(db)
	propertyService := property.NewPropertyService(propertyRepository)
	propertyHandler := property.NewPropertyHandler(propertyService)

	router := gin.Default()
	rateLimiter := limiter.NewRedisSlidingWindowLimiter(redisClient, 5, time.Minute)
	router.Use(limiter.Middleware(rateLimiter))
	routes.SetUpRoutes(router, authHandler, propertyHandler)
	router.Run(port)
}
