package api

import (
	"alcyxob/fitness-app/internal/domain" // Needed for RoleMiddleware
	"alcyxob/fitness-app/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	// swaggerFiles "github.com/swaggo/files"
	// ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRoutes(
	router *gin.Engine,
	jwtSecret string,
	authService service.AuthService,
	trainerService service.TrainerService,
	clientService service.ClientService,
	exerciseService service.ExerciseService, // Make sure this is passed in
) {

	authHandler := NewAuthHandler(authService)
	// ---> Create ExerciseHandler instance <---
	exerciseHandler := NewExerciseHandler(exerciseService)
	trainerHandler := NewTrainerHandler(trainerService)
	clientHandler := NewClientHandler(clientService)

	authMiddleware := AuthMiddleware(jwtSecret) // Using the jwtSecret parameter

	apiV1 := router.Group("/api/v1")
	{
		apiV1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong from v1"})
		})

		authGroup := apiV1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}
	}

	protected := apiV1.Group("")
	protected.Use(authMiddleware)
	{
		protected.GET("/me", func(c *gin.Context) {
			userIDStr, err := getUserIDFromContext(c)
			if err != nil {
				abortWithError(c, http.StatusInternalServerError, "Failed to get user ID from token")
				return
			}
			role, _ := getUserRoleFromContext(c)
			c.JSON(http.StatusOK, gin.H{"userId": userIDStr, "role": role})
		})

		// --- Exercise Routes ---
		exerciseGroup := protected.Group("/exercises")
		{
			// POST /api/v1/exercises - Only Trainers can create
			exerciseGroup.POST("", RoleMiddleware(domain.RoleTrainer), exerciseHandler.CreateExercise)

			// GET /api/v1/exercises - This endpoint for trainers to get their own exercises
			// The handler GetTrainerExercises uses the JWT to identify the trainer.
			// If clients also need to see exercises (e.g., a general library or ones assigned),
			// you might need a different handler or logic within GetTrainerExercises to differentiate.
			// For now, let's assume this GET is primarily for trainers.
			// If clients need access, they'd likely use it via their assignments.
			exerciseGroup.GET("", RoleMiddleware(domain.RoleTrainer), exerciseHandler.GetTrainerExercises)

			exerciseGroup.GET("/:id", exerciseHandler.GetExerciseByID) // For fetching a single exercise
			exerciseGroup.PUT("/:id", RoleMiddleware(domain.RoleTrainer), exerciseHandler.UpdateExercise)
			exerciseGroup.DELETE("/:id", RoleMiddleware(domain.RoleTrainer), exerciseHandler.DeleteExercise)

			// TODO: Add routes for specific exercise actions
			// exerciseGroup.GET("/:id", exerciseHandler.GetExerciseByID)
			// exerciseGroup.PUT("/:id", RoleMiddleware(domain.RoleTrainer), exerciseHandler.UpdateExercise)
			// exerciseGroup.DELETE("/:id", RoleMiddleware(domain.RoleTrainer), exerciseHandler.DeleteExercise)
		}

		// --- Trainer Specific Routes ---
		// All routes in this group require authentication (from 'protected')
		// AND the user to have the 'trainer' role.
		trainerApiGroup := protected.Group("/trainer")
		trainerApiGroup.Use(RoleMiddleware(domain.RoleTrainer)) // Ensure only trainers can access these
		{
			// POST /api/v1/trainer/clients
			trainerApiGroup.POST("/clients", trainerHandler.AddClientByEmail)
			// GET /api/v1/trainer/clients
			trainerApiGroup.GET("/clients", trainerHandler.GetManagedClients)

			// --- Training Plan Management ---
			// POST /api/v1/trainer/clients/{clientId}/plans
			trainerApiGroup.POST("/clients/:clientId/plans", trainerHandler.CreateTrainingPlan)
			// GET /api/v1/trainer/clients/{clientId}/plans
			trainerApiGroup.GET("/clients/:clientId/plans", trainerHandler.GetTrainingPlansForClient)

			// --- Workout Management ---
			// POST /api/v1/trainer/plans/{planId}/workouts
			trainerApiGroup.POST("/plans/:planId/workouts", trainerHandler.CreateWorkout)
			// GET /api/v1/trainer/plans/{planId}/workouts
			trainerApiGroup.GET("/plans/:planId/workouts", trainerHandler.GetWorkoutsForPlan)

			// --- Assignment Management (New Structure) ---
			// POST /api/v1/trainer/workouts/{workoutId}/exercises
			trainerApiGroup.POST("/workouts/:workoutId/exercises", trainerHandler.AssignExerciseToWorkout)

			// GET /api/v1/trainer/workouts/{workoutId}/assignments (To VIEW assignments for a workout)
			trainerApiGroup.GET("/workouts/:workoutId/assignments", trainerHandler.GetAssignmentsForWorkout)

			// GET /api/v1/trainer/assignments/{assignmentId}/video-download-url
			trainerApiGroup.GET("/assignments/:assignmentId/video-download-url", trainerHandler.GetAssignmentVideoDownloadURL)

			// PATCH /api/v1/trainer/assignments/{assignmentId}/feedback
			trainerApiGroup.PATCH("/assignments/:assignmentId/feedback", trainerHandler.SubmitFeedbackForAssignment)

			trainerApiGroup.PUT("/clients/:clientId/plans/:planId", trainerHandler.UpdateTrainingPlan)

			trainerApiGroup.DELETE("/clients/:clientId/plans/:planId", trainerHandler.DeleteTrainingPlan)

			trainerApiGroup.PUT("/plans/:planId/workouts/:workoutId", trainerHandler.UpdateWorkout)
			trainerApiGroup.DELETE("/plans/:planId/workouts/:workoutId", trainerHandler.DeleteWorkout)

			trainerApiGroup.PUT("/workouts/:workoutId/assignments/:assignmentId", trainerHandler.UpdateAssignmentInWorkout)
			trainerApiGroup.DELETE("/workouts/:workoutId/assignments/:assignmentId", trainerHandler.DeleteAssignmentFromWorkout)
		}

		clientApiGroup := protected.Group("/client")
		clientApiGroup.Use(RoleMiddleware(domain.RoleClient))
		{
			// Training Plans for the client
			clientApiGroup.GET("/plans", clientHandler.GetMyTrainingPlans)

			// Workouts for a specific plan of the client
			clientApiGroup.GET("/plans/:planId/workouts", clientHandler.GetWorkoutsForMyPlan)

			// Assignments (exercises) for a specific workout of the client
			clientApiGroup.GET("/workouts/:workoutId/assignments", clientHandler.GetAssignmentsForMyWorkout)

			clientApiGroup.PATCH("/assignments/:assignmentId/status", clientHandler.UpdateMyAssignmentStatus)
			
			// --- Routes for Upload Process ---
			clientApiGroup.POST("/assignments/:assignmentId/upload-url", clientHandler.RequestUploadURLForAssignment)
			clientApiGroup.POST("/assignments/:assignmentId/upload-confirm", clientHandler.ConfirmUploadForAssignment)

			// --- NEW Route for Logging Performance ---
			clientApiGroup.PATCH("/assignments/:assignmentId/performance", clientHandler.LogPerformanceForMyAssignment)
			clientApiGroup.GET("/workouts/today", clientHandler.GetMyCurrentWorkouts)
		}
	}
}

// Note on cfg.JWT.Secret:
// The JWT secret for AuthMiddleware is currently hardcoded or needs to be passed.
// You'll likely pass the config.JWTConfig (or just the secret string) into SetupRoutes.
// For example, modify SetupRoutes signature:
// func SetupRoutes(router *gin.Engine, jwtSecret string, authService service.AuthService, ...)
// And then in main.go: api.SetupRoutes(router, cfg.JWT.Secret, authService, ...)
// Then use that jwtSecret parameter for AuthMiddleware:
// authMiddleware := AuthMiddleware(jwtSecret)