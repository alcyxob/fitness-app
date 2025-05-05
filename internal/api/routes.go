package api

import (
	"alcyxob/fitness-app/internal/service" // Need service interfaces if handlers are methods on structs
	"net/http"

	"github.com/gin-gonic/gin"
	// Import other handlers as needed
	// swaggerFiles "github.com/swaggo/files" // swagger handler
	// ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware
)

// SetupRoutes configures the Gin router with all application routes.
func SetupRoutes(
	router *gin.Engine,
	authService service.AuthService, // Pass in all required services
	trainerService service.TrainerService,
	clientService service.ClientService,
	exerciseService service.ExerciseService,
	// Add storage service if handlers need it directly
) {

	// Create handler instances, injecting services
	authHandler := NewAuthHandler(authService)
	// exerciseHandler := NewExerciseHandler(exerciseService) // Create later
	// trainerHandler := NewTrainerHandler(trainerService, exerciseService) // Create later
	// clientHandler := NewClientHandler(clientService, exerciseService) // Create later

	// --- Middleware ---
	// Logger and Recovery middleware are added by gin.Default()
	// Add CORS middleware if needed: router.Use(cors.Default())
	authMiddleware := AuthMiddleware( /* Get JWT secret from config/service */ authService.GetJWTSecret()) // Get secret via public method

	// Basic ping/health check route
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// --- Swagger --- (Optional, requires swaggo setup)
	// router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// --- Public Routes ---
	apiV1 := router.Group("/api/v1") // Group routes under /api/v1
	{
		// Auth routes
		authGroup := apiV1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}
	} // End /api/v1 group

	// --- Protected Routes (require valid JWT) ---
	protected := apiV1.Group("")  // Continue using /api/v1 base
	protected.Use(authMiddleware) // Apply JWT authentication to all routes in this group
	{
		// Example: Route to get current user details (requires auth)
		protected.GET("/me", func(c *gin.Context) {
			userIDStr, err := getUserIDFromContext(c)
			if err != nil {
				abortWithError(c, http.StatusInternalServerError, "Failed to get user ID from token")
				return
			}
			// In a real app, fetch user details from DB using userIDStr
			// For now, just return ID and role from context
			role, _ := getUserRoleFromContext(c)
			c.JSON(http.StatusOK, gin.H{"userId": userIDStr, "role": role})
		})

		// TODO: Add routes for Exercises (requires auth)
		// exerciseGroup := protected.Group("/exercises")
		// exerciseGroup.POST("", RoleMiddleware(domain.RoleTrainer), exerciseHandler.CreateExercise) // Only Trainers
		// exerciseGroup.GET("", exerciseHandler.GetAllExercises) // Maybe allow clients too? Or filter by trainer?
		// exerciseGroup.GET("/:id", exerciseHandler.GetExerciseByID)
		// exerciseGroup.PUT("/:id", RoleMiddleware(domain.RoleTrainer), exerciseHandler.UpdateExercise) // Only Trainers
		// exerciseGroup.DELETE("/:id", RoleMiddleware(domain.RoleTrainer), exerciseHandler.DeleteExercise) // Only Trainers

		// TODO: Add routes for Trainers (require auth + trainer role)
		// trainerGroup := protected.Group("/trainer")
		// trainerGroup.Use(RoleMiddleware(domain.RoleTrainer)) // Apply Trainer role check to this group
		// {
		// 	trainerGroup.POST("/clients", trainerHandler.AddClient) // Add client by email
		//  trainerGroup.GET("/clients", trainerHandler.GetMyClients)
		// 	trainerGroup.POST("/assignments", trainerHandler.AssignExercise)
		//  trainerGroup.GET("/assignments", trainerHandler.GetMyAssignments)
		//  trainerGroup.POST("/assignments/:id/feedback", trainerHandler.SubmitFeedback)
		// }

		// TODO: Add routes for Clients (require auth + client role)
		// clientGroup := protected.Group("/client")
		// clientGroup.Use(RoleMiddleware(domain.RoleClient)) // Apply Client role check to this group
		// {
		//  clientGroup.GET("/assignments", clientHandler.GetMyAssignments)
		//  clientGroup.POST("/assignments/:id/upload-url", clientHandler.RequestUploadURL)
		//  clientGroup.POST("/assignments/:id/upload-confirm", clientHandler.ConfirmUpload)
		//  clientGroup.GET("/assignments/:id/video-url", clientHandler.GetMyVideoDownloadURL) // Get download URL for own video
		// }
	}

	// Add more route groups as needed...
}
