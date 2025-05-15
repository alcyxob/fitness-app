// internal/api/trainer_handler.go
package api

import (
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/service"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TrainerHandler struct {
	trainerService  service.TrainerService
	// exerciseService service.ExerciseService // Keep if trainer handler also manages exercise assignment
}

func NewTrainerHandler(
	trainerService service.TrainerService,
	// exerciseService service.ExerciseService,
) *TrainerHandler {
	return &TrainerHandler{
		trainerService:  trainerService,
		// exerciseService: exerciseService,
	}
}

// --- DTOs for Client Management ---
type AddClientRequest struct {
	ClientEmail string `json:"clientEmail" binding:"required,email"`
}

// (UserResponse DTO is already defined in auth_handler.go or a shared DTO file, we can reuse it)

// --- Handler Methods for Client Management ---

// AddClientByEmail godoc
// @Summary Add a client to the trainer's roster by email
// @Description Associates an existing client user with the authenticated trainer.
// @Tags Trainer
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param clientRequest body AddClientRequest true "Client's email"
// @Success 200 {object} UserResponse "Client successfully added/associated"
// @Failure 400 {object} gin.H "Invalid input (validation error, or invalid trainer ID in token)"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer, or client already has a trainer, or user is not a client)"
// @Failure 404 {object} gin.H "Client not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /trainer/clients [post]
func (h *TrainerHandler) AddClientByEmail(c *gin.Context) {
	var req AddClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "Validation error: "+err.Error())
		return
	}

	trainerIDStr, err := getUserIDFromContext(c) // Helper from middleware.go
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer from token.")
		return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
		return
	}

	client, err := h.trainerService.AddClientByEmail(c.Request.Context(), trainerID, req.ClientEmail)
	if err != nil {
		// Map service errors to HTTP status codes
		if errors.Is(err, service.ErrClientNotFound) {
			abortWithError(c, http.StatusNotFound, err.Error())
		} else if errors.Is(err, service.ErrClientNotRole) || errors.Is(err, service.ErrClientAlreadyAssigned) {
			abortWithError(c, http.StatusForbidden, err.Error()) // Or StatusConflict for ErrClientAlreadyAssigned
		} else {
			// log.Printf("Error adding client by email: %v", err) // Server-side logging
			abortWithError(c, http.StatusInternalServerError, "Failed to add client.")
		}
		return
	}

	c.JSON(http.StatusOK, MapUserToResponse(client)) // Reuse MapUserToResponse from auth_handler
}

// GetManagedClients godoc
// @Summary Get the trainer's managed clients
// @Description Retrieves a list of clients currently managed by the authenticated trainer.
// @Tags Trainer
// @Produce json
// @Security BearerAuth
// @Success 200 {array} UserResponse "List of managed clients"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer)"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /trainer/clients [get]
func (h *TrainerHandler) GetManagedClients(c *gin.Context) {
	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer from token.")
		return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
		return
	}

	clients, err := h.trainerService.GetManagedClients(c.Request.Context(), trainerID)
	if err != nil {
		// log.Printf("Error getting managed clients: %v", err) // Server-side logging
		abortWithError(c, http.StatusInternalServerError, "Failed to retrieve managed clients.")
		return
	}

	if clients == nil {
		c.JSON(http.StatusOK, []UserResponse{}) // Return empty JSON array, not null
		return
	}

	c.JSON(http.StatusOK, MapUsersToResponse(clients)) // Need MapUsersToResponse helper
}


// MapUsersToResponse helper (similar to MapExercisesToResponse)
// You can place this in a shared DTO utility file or keep it here if only used by TrainerHandler.
// For now, let's assume it's similar to the one in auth_handler.
// If MapUserToResponse is in auth_handler.go and not exported from api package,
// you might need to make it public or duplicate it.
// Let's assume MapUserToResponse from auth_handler can be used if this handler is in the same package `api`.
// We'll need a MapUsersToResponse.

// MapUsersToResponse converts a slice of domain.User to UserResponse DTOs.
func MapUsersToResponse(users []domain.User) []UserResponse {
	userResponses := make([]UserResponse, len(users))
	for i, u := range users {
		userResponses[i] = MapUserToResponse(&u) // Assuming MapUserToResponse is accessible
	}
	return userResponses
}


// TODO: Implement handlers for AssignExercise, GetAssignmentsByTrainer (for trainer), SubmitFeedback
// These will use h.trainerService and potentially h.exerciseService

// --- DTOs for Assignment Management ---

// AssignExerciseRequest defines the payload for assigning an exercise.
type AssignExerciseRequest struct {
	ClientID   string     `json:"clientId" binding:"required"` // Client's ObjectID hex string
	ExerciseID string     `json:"exerciseId" binding:"required"` // Exercise's ObjectID hex string
	DueDate    *time.Time `json:"dueDate"`                   // Optional pointer for due date (e.g., "2024-12-31T23:59:59Z")
}

// AssignmentResponse is the DTO for returning assignment details.
type AssignmentResponse struct {
	ID         string    `json:"id"`
	WorkoutID  string    `json:"workoutId"`  // Link to Workout
	ExerciseID string    `json:"exerciseId"` // Link to Exercise
	AssignedAt time.Time `json:"assignedAt"`
	Status     string    `json:"status"`
	// Execution details
	Sets         *int    `json:"sets,omitempty"`
	Reps         *string `json:"reps,omitempty"`
	Rest         *string `json:"rest,omitempty"`
	Tempo        *string `json:"tempo,omitempty"`
	Weight       *string `json:"weight,omitempty"`
	Duration     *string `json:"duration,omitempty"`
	Sequence     int     `json:"sequence"`
	TrainerNotes string  `json:"trainerNotes,omitempty"`
	// Client tracking
	ClientNotes string  `json:"clientNotes,omitempty"`
	UploadID    *string `json:"uploadId,omitempty"`
	Feedback    string  `json:"feedback,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
    // REMOVED: ClientID, TrainerID, DueDate
}

// MapAssignmentToResponse converts domain.Assignment to AssignmentResponse DTO
func MapAssignmentToResponse(a *domain.Assignment) AssignmentResponse {
	if a == nil {
		return AssignmentResponse{}
	}
	var uploadIDHex *string
	if a.UploadID != nil && *a.UploadID != primitive.NilObjectID {
		hex := (*a.UploadID).Hex()
		uploadIDHex = &hex
	}
	return AssignmentResponse{
		ID:         a.ID.Hex(),
		WorkoutID:  a.WorkoutID.Hex(), // Use WorkoutID
		ExerciseID: a.ExerciseID.Hex(),
		AssignedAt: a.AssignedAt,
		Status:     string(a.Status),
		Sets:       a.Sets,
		Reps:       a.Reps,
		Rest:       a.Rest,
		Tempo:      a.Tempo,
		Weight:     a.Weight,
		Duration:   a.Duration,
		Sequence:   a.Sequence,
        TrainerNotes: a.TrainerNotes,
		ClientNotes: a.ClientNotes,
		UploadID:    uploadIDHex,
		Feedback:    a.Feedback,
		UpdatedAt:   a.UpdatedAt,
        // REMOVED: ClientID, TrainerID, DueDate assignments
	}
}

// MapAssignmentsToResponse converts a slice of domain.Assignment
func MapAssignmentsToResponse(assignments []domain.Assignment) []AssignmentResponse {
	responses := make([]AssignmentResponse, len(assignments))
	for i, a := range assignments {
		responses[i] = MapAssignmentToResponse(&a)
	}
	return responses
}

// --- DTOs for Training Plan Management ---

type CreateTrainingPlanRequest struct {
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	StartDate   *time.Time `json:"startDate"` // Expect ISO8601 format string e.g., "2024-05-10T00:00:00Z"
	EndDate     *time.Time `json:"endDate"`
	IsActive    bool       `json:"isActive"` // Defaults to false if omitted
}

type TrainingPlanResponse struct {
	ID          string     `json:"id"`
	TrainerID   string     `json:"trainerId"`
	ClientID    string     `json:"clientId"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	StartDate   *time.Time `json:"startDate,omitempty"`
	EndDate     *time.Time `json:"endDate,omitempty"`
	IsActive    bool       `json:"isActive"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// MapTrainingPlanToResponse converts domain.TrainingPlan to DTO
func MapTrainingPlanToResponse(p *domain.TrainingPlan) TrainingPlanResponse {
	if p == nil {
		return TrainingPlanResponse{}
	}
	return TrainingPlanResponse{
		ID:          p.ID.Hex(),
		TrainerID:   p.TrainerID.Hex(),
		ClientID:    p.ClientID.Hex(),
		Name:        p.Name,
		Description: p.Description,
		StartDate:   p.StartDate,
		EndDate:     p.EndDate,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// MapTrainingPlansToResponse converts a slice of domain.TrainingPlan
func MapTrainingPlansToResponse(plans []domain.TrainingPlan) []TrainingPlanResponse {
	responses := make([]TrainingPlanResponse, len(plans))
	for i, p := range plans {
		responses[i] = MapTrainingPlanToResponse(&p)
	}
	return responses
}

// --- Handler Methods for Training Plan Management ---

// CreateTrainingPlan godoc
// @Summary Create a new training plan for a client
// @Description Creates a training plan for a specific client managed by the authenticated trainer.
// @Tags Trainer Plans
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param clientId path string true "Client's ObjectID Hex"
// @Param planRequest body CreateTrainingPlanRequest true "Training Plan details"
// @Success 201 {object} TrainingPlanResponse "Training plan created successfully"
// @Failure 400 {object} gin.H "Invalid input (validation error, invalid client ID)"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer, or client not managed by this trainer)"
// @Failure 404 {object} gin.H "Client not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /trainer/clients/{clientId}/plans [post]
func (h *TrainerHandler) CreateTrainingPlan(c *gin.Context) {
	// Get client ID from path parameter
	clientIDHex := c.Param("clientId")
	clientID, err := primitive.ObjectIDFromHex(clientIDHex)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid client ID format in URL path.")
		return
	}

	// Bind request body
	var req CreateTrainingPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "Validation error: "+err.Error())
		return
	}

	// Get trainer ID from token
	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer from token.")
		return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
		return
	}

	// Call service
	plan, err := h.trainerService.CreateTrainingPlan(
		c.Request.Context(),
		trainerID,
		clientID,
		req.Name,
		req.Description,
		req.StartDate, // Pass pointers directly
		req.EndDate,
		req.IsActive,
	)
	if err != nil {
		if errors.Is(err, service.ErrClientNotFound) {
			abortWithError(c, http.StatusNotFound, err.Error())
		} else if errors.Is(err, service.ErrClientNotManaged) {
			abortWithError(c, http.StatusForbidden, err.Error())
        } else if errors.Is(err, service.ErrTrainingPlanCreationFailed) {
            abortWithError(c, http.StatusInternalServerError, err.Error())
		} else {
			// log.Printf("Error creating training plan: %v", err)
			abortWithError(c, http.StatusInternalServerError, "Failed to create training plan.")
		}
		return
	}

	c.JSON(http.StatusCreated, MapTrainingPlanToResponse(plan))
}

// GetTrainingPlansForClient godoc
// @Summary Get training plans for a specific client
// @Description Retrieves all training plans created by the authenticated trainer for a specific client.
// @Tags Trainer Plans
// @Produce json
// @Security BearerAuth
// @Param clientId path string true "Client's ObjectID Hex"
// @Success 200 {array} TrainingPlanResponse "List of training plans"
// @Failure 400 {object} gin.H "Invalid client ID format"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer, or client not managed)"
// @Failure 404 {object} gin.H "Client not found (less likely if client ID is valid)"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /trainer/clients/{clientId}/plans [get]
func (h *TrainerHandler) GetTrainingPlansForClient(c *gin.Context) {
	// Get client ID from path parameter
	clientIDHex := c.Param("clientId")
	clientID, err := primitive.ObjectIDFromHex(clientIDHex)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid client ID format in URL path.")
		return
	}

	// Get trainer ID from token
	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer from token.")
		return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
		return
	}

	// Call service
	plans, err := h.trainerService.GetTrainingPlansForClient(c.Request.Context(), trainerID, clientID)
	if err != nil {
        // Service layer currently returns generic error, could be more specific
		// log.Printf("Error fetching plans for client %s: %v", clientIDHex, err)
		abortWithError(c, http.StatusInternalServerError, "Failed to retrieve training plans.")
		return
	}

	// Return empty array if no plans found (not an error)
	if plans == nil {
        c.JSON(http.StatusOK, []TrainingPlanResponse{})
        return
    }

	c.JSON(http.StatusOK, MapTrainingPlansToResponse(plans))
}

// --- DTOs for Workout Management ---

type CreateWorkoutRequest struct {
	Name      string `json:"name" binding:"required"`
	DayOfWeek *int   `json:"dayOfWeek" binding:"omitempty,min=1,max=7"` // Optional day
	Notes     string `json:"notes"`
	Sequence  *int    `json:"sequence" binding:"required,min=0"` // Require sequence
}

type WorkoutResponse struct {
	ID             string     `json:"id"`
	TrainingPlanID string     `json:"trainingPlanId"`
	TrainerID      string     `json:"trainerId"`
	ClientID       string     `json:"clientId"`
	Name           string     `json:"name"`
	DayOfWeek      *int       `json:"dayOfWeek,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	Sequence       int        `json:"sequence"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// MapWorkoutToResponse converts domain.Workout to DTO
func MapWorkoutToResponse(w *domain.Workout) WorkoutResponse {
	if w == nil {
		return WorkoutResponse{}
	}
	return WorkoutResponse{
		ID:             w.ID.Hex(),
		TrainingPlanID: w.TrainingPlanID.Hex(),
		TrainerID:      w.TrainerID.Hex(),
		ClientID:       w.ClientID.Hex(),
		Name:           w.Name,
		DayOfWeek:      w.DayOfWeek,
		Notes:          w.Notes,
		Sequence:       w.Sequence,
		CreatedAt:      w.CreatedAt,
		UpdatedAt:      w.UpdatedAt,
	}
}

// MapWorkoutsToResponse converts a slice of domain.Workout
func MapWorkoutsToResponse(workouts []domain.Workout) []WorkoutResponse {
	responses := make([]WorkoutResponse, len(workouts))
	for i, w := range workouts {
		responses[i] = MapWorkoutToResponse(&w)
	}
	return responses
}

// --- Handler Methods for Workout Management ---

// CreateWorkout godoc
// @Summary Create a new workout within a training plan
// @Description Creates a workout session associated with a specific training plan owned by the trainer.
// @Tags Trainer Workouts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param planId path string true "Training Plan's ObjectID Hex"
// @Param workoutRequest body CreateWorkoutRequest true "Workout details"
// @Success 201 {object} WorkoutResponse "Workout created successfully"
// @Failure 400 {object} gin.H "Invalid input (validation error, invalid plan ID)"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer, or does not own the plan)"
// @Failure 404 {object} gin.H "Training Plan not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /trainer/plans/{planId}/workouts [post]
func (h *TrainerHandler) CreateWorkout(c *gin.Context) {
	// Get plan ID from path parameter
	planIDHex := c.Param("planId")
	planID, err := primitive.ObjectIDFromHex(planIDHex)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid training plan ID format in URL path.")
		return
	}

	// Bind request body
	var req CreateWorkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "Validation error: "+err.Error())
		return
	}

	// Get trainer ID from token
	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer from token.")
		return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
		return
	}

	// Call service
	var sequenceVal int
	if req.Sequence != nil { // Check if the pointer is not nil
			sequenceVal = *req.Sequence
	} else {
			// Handle case where sequence is truly not provided, though "required" should prevent this
			// For "required" on a pointer, it means the key must be present in JSON, even if value is null.
			// If JSON sends "sequence": null, req.Sequence will be nil.
			// If JSON omits "sequence", req.Sequence will be nil.
			// If "required" means "key must exist and value must not be the zero-value for the pointed-to type (e.g. not 0 for int)"
			// then sending "sequence": 0 would be fine.
			// The issue is the default validator's behavior with "required" on non-pointer int.
			// Let's assume if it passes binding, and req.Sequence is not nil, we use its value.
			// If binding:"required" on *int means "key must exist and value must be provided (not null)", then this is fine.
			// If `req.Sequence` could be nil after binding and that's an error, you'd check here.
			// But given the error, the issue is with `int` and `0`.
			// If req.Sequence is nil here despite being required, Gin's binding itself has an issue with *int and required.
			// Let's assume required *int means it will be non-nil if validation passes.
			sequenceVal = *req.Sequence // If required *int guarantees non-nil, this is safe.
	}

	workout, err := h.trainerService.CreateWorkout(
		c.Request.Context(),
		trainerID,
		planID,
		req.Name,
		req.DayOfWeek,
		req.Notes,
		sequenceVal,
	)
	if err != nil {
		// Map service errors
		if errors.Is(err, service.ErrTrainingPlanNotFound) {
			abortWithError(c, http.StatusNotFound, err.Error())
		} else if errors.Is(err, service.ErrTrainingPlanAccessDenied) {
			abortWithError(c, http.StatusForbidden, err.Error())
        } else if errors.Is(err, service.ErrWorkoutCreationFailed) {
            abortWithError(c, http.StatusInternalServerError, err.Error())
		} else {
			// log.Printf("Error creating workout: %v", err)
			abortWithError(c, http.StatusInternalServerError, "Failed to create workout.")
		}
		return
	}

	c.JSON(http.StatusCreated, MapWorkoutToResponse(workout))
}

// GetWorkoutsForPlan godoc
// @Summary Get workouts for a specific training plan
// @Description Retrieves all workouts associated with a specific training plan owned by the trainer.
// @Tags Trainer Workouts
// @Produce json
// @Security BearerAuth
// @Param planId path string true "Training Plan's ObjectID Hex"
// @Success 200 {array} WorkoutResponse "List of workouts"
// @Failure 400 {object} gin.H "Invalid plan ID format"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer, or does not own the plan)"
// @Failure 404 {object} gin.H "Training Plan not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /trainer/plans/{planId}/workouts [get]
func (h *TrainerHandler) GetWorkoutsForPlan(c *gin.Context) {
	// Get plan ID from path parameter
	planIDHex := c.Param("planId")
	planID, err := primitive.ObjectIDFromHex(planIDHex)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid training plan ID format in URL path.")
		return
	}

	// Get trainer ID from token
	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer from token.")
		return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
		return
	}

	// Call service
	workouts, err := h.trainerService.GetWorkoutsForPlan(c.Request.Context(), trainerID, planID)
	if err != nil {
        // Map service errors
        if errors.Is(err, service.ErrTrainingPlanNotFound) {
			abortWithError(c, http.StatusNotFound, err.Error())
		} else if errors.Is(err, service.ErrTrainingPlanAccessDenied) {
            abortWithError(c, http.StatusForbidden, err.Error())
        } else {
		    // log.Printf("Error fetching workouts for plan %s: %v", planIDHex, err)
		    abortWithError(c, http.StatusInternalServerError, "Failed to retrieve workouts.")
        }
		return
	}

    if workouts == nil {
        c.JSON(http.StatusOK, []WorkoutResponse{})
        return
    }

	c.JSON(http.StatusOK, MapWorkoutsToResponse(workouts))
}

// --- DTO for Assigning Exercise to Workout ---

type AssignExerciseToWorkoutRequest struct {
	ExerciseID   string  `json:"exerciseId" binding:"required"` // Exercise ObjectID hex
	Sets         *int    `json:"sets" binding:"omitempty"`                          // Optional pointer allows distinguishing 0 from omitted
	Reps         *string `json:"reps" binding:"omitempty"`                          // e.g., "8-12", "AMRAP"
	Rest         *string `json:"rest" binding:"omitempty"`                          // e.g., "60s", "2m"
	Tempo        *string `json:"tempo" binding:"omitempty"`                         // e.g., "2010"
	Weight       *string `json:"weight" binding:"omitempty"`                        // e.g., "10kg", "BW", "RPE 8"
	Duration     *string `json:"duration" binding:"omitempty"`                      // e.g., "30min", "5km"
	Sequence     *int     `json:"sequence" binding:"required,min=0"` // Order within workout
	TrainerNotes string  `json:"trainerNotes" binding:"omitempty"`
	// Note: We don't include WorkoutID in the *body* because it's in the URL path.
}

// --- Handler Method for Assigning Exercise to Workout ---

// AssignExerciseToWorkout godoc
// @Summary Assign an exercise to a specific workout
// @Description Adds an exercise with specific parameters (sets, reps, etc.) to a workout within a plan owned by the trainer.
// @Tags Trainer Workouts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param workoutId path string true "Workout's ObjectID Hex"
// @Param assignmentRequest body AssignExerciseToWorkoutRequest true "Exercise assignment details"
// @Success 201 {object} AssignmentResponse "Exercise assigned successfully to workout"
// @Failure 400 {object} gin.H "Invalid input (validation error, invalid IDs)"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer, or does not own the workout/exercise)"
// @Failure 404 {object} gin.H "Workout or Exercise not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /trainer/workouts/{workoutId}/exercises [post]
func (h *TrainerHandler) AssignExerciseToWorkout(c *gin.Context) {
	// Get workout ID from path parameter
	workoutIDHex := c.Param("workoutId")
	workoutID, err := primitive.ObjectIDFromHex(workoutIDHex)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid workout ID format in URL path.")
		return
	}

	// Bind request body
	var req AssignExerciseToWorkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "Validation error: "+err.Error())
		return
	}

	// Get trainer ID from token
	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer from token.")
		return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
		return
	}

	// Convert ExerciseID from request
	exerciseID, err := primitive.ObjectIDFromHex(req.ExerciseID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid exercise ID format.")
		return
	}

    // Construct the domain.Assignment object from the request DTO
    var sequenceVal int
    if req.Sequence == nil {
        // This case should ideally be caught by `binding:"required"` if it means "not null".
        // If "required" on *int only means "key must be present", then JSON `{"sequence": null}`
        // would result in req.Sequence being nil.
        // For now, if it passes validation, we assume req.Sequence is not nil.
        // If it could be nil, you need to decide what that means (e.g., default to 0 or error).
        // Given the "required" tag, a nil pointer here would mean the binding is not enforcing non-null.
        abortWithError(c, http.StatusBadRequest, "Validation error: Sequence is required but was null or not provided correctly.")
        return
    }
    sequenceVal = *req.Sequence

    assignmentDetails := domain.Assignment{
        // WorkoutID and ExerciseID will be set/validated by the service
        Sets:           req.Sets,
        Reps:           req.Reps,
        Rest:           req.Rest,
        Tempo:          req.Tempo,
        Weight:         req.Weight,
        Duration:       req.Duration,
        Sequence:       sequenceVal,
        TrainerNotes:   req.TrainerNotes,
        // Status will default in repo/service, other fields are for client interaction
    }


	// Call the service
	createdAssignment, err := h.trainerService.AssignExerciseToWorkout(
		c.Request.Context(),
		trainerID,
		workoutID,
		exerciseID,
		assignmentDetails,
	)
	if err != nil {
		// Map service errors
		if errors.Is(err, service.ErrWorkoutNotFound) || errors.Is(err, service.ErrExerciseNotFound) {
			abortWithError(c, http.StatusNotFound, err.Error())
		} else if errors.Is(err, service.ErrTrainingPlanAccessDenied) || errors.Is(err, service.ErrExerciseAccessDenied) || errors.Is(err, errors.New("access denied: trainer does not own this workout")) { // Crude check for now
            abortWithError(c, http.StatusForbidden, err.Error())
        } else {
			// log.Printf("Error assigning exercise to workout: %v", err)
			abortWithError(c, http.StatusInternalServerError, "Failed to assign exercise.")
		}
		return
	}

	// Respond with the created assignment DTO
	c.JSON(http.StatusCreated, MapAssignmentToResponse(createdAssignment))
}

// GetAssignmentsForWorkout godoc
// @Summary Get all assigned exercises for a specific workout
// @Description Retrieves all exercises assigned to a particular workout owned by the trainer.
// @Tags Trainer Workouts
// @Produce json
// @Security BearerAuth
// @Param workoutId path string true "Workout's ObjectID Hex"
// @Success 200 {array} AssignmentResponse "List of assignments (exercises with parameters)"
// @Failure 400 {object} gin.H "Invalid workout ID format"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (not a trainer, or does not own the workout)"
// @Failure 404 {object} gin.H "Workout not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /trainer/workouts/{workoutId}/assignments [get]
func (h *TrainerHandler) GetAssignmentsForWorkout(c *gin.Context) {
	workoutIDHex := c.Param("workoutId")
	workoutID, err := primitive.ObjectIDFromHex(workoutIDHex)
	if err != nil {
			abortWithError(c, http.StatusBadRequest, "Invalid workout ID format in URL path.")
			return
	}

	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
			abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer.")
			return
	}
	trainerID, err := primitive.ObjectIDFromHex(trainerIDStr)
	if err != nil {
			abortWithError(c, http.StatusBadRequest, "Invalid trainer ID format in token.")
			return
	}

	assignments, err := h.trainerService.GetAssignmentsForWorkout(c.Request.Context(), trainerID, workoutID)
	if err != nil {
			if errors.Is(err, service.ErrWorkoutNotFound) {
					abortWithError(c, http.StatusNotFound, err.Error())
			} else if errors.Is(err, errors.New("access denied: trainer does not own this workout")) { // Crude check
					abortWithError(c, http.StatusForbidden, err.Error())
			} else {
					abortWithError(c, http.StatusInternalServerError, "Failed to retrieve assignments.")
			}
			return
	}

	if assignments == nil {
			c.JSON(http.StatusOK, []AssignmentResponse{})
			return
	}
	c.JSON(http.StatusOK, MapAssignmentsToResponse(assignments))
}

// --- DTO for Video Download URL Response ---
type VideoDownloadURLResponse struct {
	DownloadURL string `json:"downloadUrl"`
}

// --- Handler Method for Getting Video Download URL ---

// GetAssignmentVideoDownloadURL godoc
// @Summary Get a pre-signed download URL for a client's assignment video
// @Description Allows a trainer to get a temporary URL to view a video uploaded by a client for an assignment.
// @Tags Trainer Assignments
// @Produce json
// @Security BearerAuth
// @Param assignmentId path string true "Assignment's ObjectID Hex"
// @Success 200 {object} VideoDownloadURLResponse "Pre-signed S3 URL for video download"
// @Failure 400 {object} gin.H "Invalid assignment ID format"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (trainer does not own this assignment/workout)"
// @Failure 404 {object} gin.H "Assignment or Upload not found"
// @Failure 500 {object} gin.H "Internal Server Error (e.g., S3 error)"
// @Router /trainer/assignments/{assignmentId}/video-download-url [get]
func (h *TrainerHandler) GetAssignmentVideoDownloadURL(c *gin.Context) {
	trainerIDStr, err := getUserIDFromContext(c)
	if err != nil {
			abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer.")
			return
	}
	trainerID, _ := primitive.ObjectIDFromHex(trainerIDStr) // Assume valid from token

	assignmentIDHex := c.Param("assignmentId")
	assignmentID, err := primitive.ObjectIDFromHex(assignmentIDHex)
	if err != nil {
			abortWithError(c, http.StatusBadRequest, "Invalid assignment ID format.")
			return
	}

	downloadURL, err := h.trainerService.GetAssignmentVideoDownloadURL(c.Request.Context(), trainerID, assignmentID)
	if err != nil {
			// Map service errors to HTTP status codes
			if errors.Is(err, service.ErrAssignmentNotFound) || errors.Is(err, service.ErrWorkoutNotFound) || errors.Is(err, service.ErrUploadNotFoundForAssignment) {
					abortWithError(c, http.StatusNotFound, err.Error())
			} else if errors.Is(err, service.ErrAssignmentAccessDenied) {
					abortWithError(c, http.StatusForbidden, err.Error())
			} else if errors.Is(err, service.ErrS3URLGenerationFailed) {
					 abortWithError(c, http.StatusInternalServerError, "Could not generate video URL.")
			} else {
					// log.Printf("Error getting video download URL for assignment %s: %v", assignmentIDHex, err)
					abortWithError(c, http.StatusInternalServerError, "Failed to get video download URL.")
			}
			return
	}

	c.JSON(http.StatusOK, VideoDownloadURLResponse{DownloadURL: downloadURL})
}

// --- DTO for Submitting Feedback ---
type SubmitFeedbackRequest struct {
    Feedback string `json:"feedback"` // Can be empty if only status changes
    Status   string `json:"status" binding:"required"` // New status, e.g., "reviewed"
}


// --- Handler Method for Submitting Feedback ---

// SubmitFeedbackForAssignment godoc
// @Summary Submit feedback and update status for a client's assignment
// @Description Allows a trainer to provide feedback on a submitted assignment and change its status.
// @Tags Trainer Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assignmentId path string true "Assignment's ObjectID Hex"
// @Param feedbackRequest body SubmitFeedbackRequest true "Feedback and new status"
// @Success 200 {object} AssignmentResponse "Feedback submitted and assignment updated"
// @Failure 400 {object} gin.H "Invalid input (validation error, invalid ID, invalid status)"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (trainer does not own assignment, or invalid status transition)"
// @Failure 404 {object} gin.H "Assignment or associated Workout not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /trainer/assignments/{assignmentId}/feedback [patch]
func (h *TrainerHandler) SubmitFeedbackForAssignment(c *gin.Context) {
    trainerIDStr, err := getUserIDFromContext(c)
    if err != nil {
        abortWithError(c, http.StatusUnauthorized, "Unable to identify trainer.")
        return
    }
    trainerID, _ := primitive.ObjectIDFromHex(trainerIDStr) // Assume valid from token

    assignmentIDHex := c.Param("assignmentId")
    assignmentID, err := primitive.ObjectIDFromHex(assignmentIDHex)
    if err != nil {
        abortWithError(c, http.StatusBadRequest, "Invalid assignment ID format.")
        return
    }

    var req SubmitFeedbackRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        abortWithError(c, http.StatusBadRequest, "Validation error: "+err.Error())
        return
    }

    // Convert string status from request to domain.AssignmentStatus
    // Add validation here for allowed status strings from trainer
    newDomainStatus := domain.AssignmentStatus(req.Status)
    // Example: Trainer can set to "reviewed" or "assigned" (for re-do)
    if newDomainStatus != domain.StatusReviewed && newDomainStatus != domain.StatusAssigned {
         abortWithError(c, http.StatusBadRequest, "Invalid target status provided by trainer.")
         return
    }


    updatedAssignment, err := h.trainerService.SubmitFeedback(
        c.Request.Context(),
        trainerID,
        assignmentID,
        req.Feedback,
        newDomainStatus,
    )
    if err != nil {
        // Map service errors
        if errors.Is(err, service.ErrAssignmentNotFound) || errors.Is(err, service.ErrWorkoutNotFound) {
            abortWithError(c, http.StatusNotFound, err.Error())
        } else if errors.Is(err, service.ErrAssignmentAccessDenied) || errors.Is(err, service.ErrInvalidAssignmentStatusUpdate) {
            abortWithError(c, http.StatusForbidden, err.Error())
        } else {
            // log.Printf("Error submitting feedback for assignment %s: %v", assignmentIDHex, err)
            abortWithError(c, http.StatusInternalServerError, "Failed to submit feedback.")
        }
        return
    }

    c.JSON(http.StatusOK, MapAssignmentToResponse(updatedAssignment))
}