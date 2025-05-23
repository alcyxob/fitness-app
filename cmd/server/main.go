package main

import (
	"alcyxob/fitness-app/internal/api" // Import API package
	"alcyxob/fitness-app/internal/config"
	"alcyxob/fitness-app/internal/repository/mongo"
	"alcyxob/fitness-app/internal/service"
	"alcyxob/fitness-app/internal/storage"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// @title Fitness Trainer API
// @version 1.0
// @description API for managing fitness trainers, clients, exercises, and assignments.
// @contact.name API Support
// @contact.email support@example.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	log.Println("Starting Fitness App Server...")
	log.Println("---- DUMPING ALL ENVIRONMENT VARIABLES ----")
	for _, e := range os.Environ() {
			pair := strings.SplitN(e, "=", 2)
			// Only print relevant ones or be careful with printing all in production logs if sensitive
			if strings.HasPrefix(pair[0], "JWT_") || strings.HasPrefix(pair[0], "S3_") || strings.HasPrefix(pair[0], "DATABASE_") || strings.HasPrefix(pair[0], "SERVER_") {
					log.Printf("ENV: %s = %s", pair[0], pair[1])
			}
	}
	log.Println("---- FINISHED DUMPING ENV VARS ----")

	// --- Configuration ---
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("FATAL: Could not load config: %v", err)
	}
	log.Println("Configuration loaded.")

	// --- Database Connection ---
	dbClient, err := mongo.ConnectDB(cfg.Database.URI)
	if err != nil {
		log.Fatalf("FATAL: Could not connect to MongoDB: %v", err)
	}
	defer func() {
		log.Println("Disconnecting MongoDB...")
		if err := mongo.DisconnectDB(dbClient); err != nil {
			log.Printf("ERROR: Failed to disconnect MongoDB: %v", err)
		}
	}()
	appDB := dbClient.Database(cfg.Database.Name)
	log.Println("Database connection established.")

	// --- Ensure Indexes ---
	log.Println("Ensuring database indexes...")
	go func() { // Run index creation concurrently/in background
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute) // Timeout for index creation
		defer cancel()
		mongo.EnsureUserIndexes(ctx, appDB.Collection("users"))
		mongo.EnsureExerciseIndexes(ctx, appDB.Collection("exercises"))
		mongo.EnsureAssignmentIndexes(ctx, appDB.Collection("assignments"))
		mongo.EnsureUploadIndexes(ctx, appDB.Collection("uploads"))
		mongo.EnsureTrainingPlanIndexes(ctx, appDB.Collection("training_plans"))
		mongo.EnsureWorkoutIndexes(ctx, appDB.Collection("workouts"))
		log.Println("Index creation process completed.")
	}()

	// --- Initialize Storage ---
	log.Println("Initializing file storage service...")
	fileStorage, err := storage.NewS3Storage(cfg.S3)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize S3 storage: %v", err)
	}

	// --- Initialize Repositories ---
	log.Println("Initializing repositories...")
	userRepo := mongo.NewMongoUserRepository(appDB)
	exerciseRepo := mongo.NewMongoExerciseRepository(appDB)
	assignmentRepo := mongo.NewMongoAssignmentRepository(appDB)
	uploadRepo := mongo.NewMongoUploadRepository(appDB)
	trainingPlanRepo := mongo.NewMongoTrainingPlanRepository(appDB) // ADDED
	workoutRepo := mongo.NewMongoWorkoutRepository(appDB)
  // workoutRepo := mongo.NewMongoWorkoutRepository(appDB) // Add later

	// --- Initialize Services ---
	log.Println("Initializing services...")
	// Pass JWT config directly
	authService := service.NewAuthService(userRepo, cfg.JWT.Secret, cfg.JWT.Expiration)
	exerciseService := service.NewExerciseService(exerciseRepo)
	trainerService := service.NewTrainerService(userRepo, assignmentRepo, exerciseRepo, trainingPlanRepo, workoutRepo, uploadRepo, fileStorage)
	clientService := service.NewClientService(userRepo, assignmentRepo, uploadRepo, exerciseRepo, workoutRepo, trainingPlanRepo, fileStorage)

	// --- Initialize Gin Engine ---
	// gin.SetMode(gin.ReleaseMode) // Uncomment for production
	router := gin.Default() // Includes Logger and Recovery middleware

	// --- Setup Routes ---
	log.Println("Setting up API routes...")
	// Pass services to the route setup function
	api.SetupRoutes(router, cfg.JWT.Secret, authService, trainerService, clientService, exerciseService)

	// --- Start HTTP Server ---
	server := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Server starting on %s", cfg.Server.Address)

	// --- Graceful Shutdown ---
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("FATAL: ListenAndServe Error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the requests it is currently handling
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("FATAL: Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting.")
}
