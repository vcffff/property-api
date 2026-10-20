package main

import (
	"dev/api-task-manager/configs"
	"dev/api-task-manager/internal/features/application"
	"dev/api-task-manager/internal/features/auth"
	"dev/api-task-manager/internal/features/property"
	"dev/api-task-manager/internal/features/user"
	"dev/api-task-manager/internal/features/viewing"
	"dev/api-task-manager/internal/infrastructure/redis"
	"dev/api-task-manager/internal/platform/database"
	"dev/api-task-manager/internal/platform/kafka"
	"dev/api-task-manager/internal/platform/middleware/limiter"
	"dev/api-task-manager/internal/routes"
	"dev/api-task-manager/internal/session"
	"fmt"
	"log"
	"os"
	"strings"
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
		&application.Application{},
		&viewing.Viewing{},
	); err != nil {
		log.Fatal("Migration failed:", err)
	}
	if err := application.EnsureActiveApplicationIndex(db); err != nil {
		log.Fatal("Application migration failed:", err)
	}
	if err := viewing.EnsureActiveSlotIndex(db); err != nil {
		log.Fatal("Viewing migration failed:", err)
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

	var brokers []string
	for _, broker := range strings.Split(os.Getenv("KAFKA_BROKERS"), ",") {
		if broker = strings.TrimSpace(broker); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	if len(brokers) == 0 {
		brokers = []string{"127.0.0.1:9092"}
	}
	producer := kafka.NewProducer([]string{cfg.KafkaBrokers})
	defer func() {
		if err := producer.Close(); err != nil {
			log.Printf("Kafka producer close failed: %v", err)
		}
	}()

	propertyRepository := property.NewPropertyRepository(db)
	propertyService := property.NewPropertyService(propertyRepository, producer)
	propertyHandler := property.NewPropertyHandler(propertyService)

	applicationRepository := application.NewApplicationRepository(db)
	applicationService := application.NewApplicationService(applicationRepository)
	applicationHandler := application.NewApplicationHandler(applicationService)

	viewingRepo := viewing.NewViewingRepository(db)
	viewingService := viewing.NewViewingService(viewingRepo)
	viewingHandler := viewing.NewViewingHandler(viewingService)

	router := gin.Default()
	rateLimiter := limiter.NewRedisSlidingWindowLimiter(redisClient, 5, time.Minute)
	router.Use(limiter.Middleware(rateLimiter))
	routes.SetUpRoutes(router, authHandler, propertyHandler, applicationHandler, viewingHandler)
	router.Run(port)
}
