// internal/api/client_handler.go
package api

import (
	"alcyxob/fitness-app/internal/domain"
	"alcyxob/fitness-app/internal/service"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ClientHandler struct {
	clientService service.ClientService
	// authService service.AuthService // For getting clientID from token
}

func NewClientHandler(clientService service.ClientService) *ClientHandler {
	return &ClientHandler{clientService: clientService}
}

// --- DTOs (Reuse existing where possible) ---
// TrainingPlanResponse is already in trainer_handler.go (or a shared DTO file)
// WorkoutResponse is already in trainer_handler.go (or a shared DTO file)
// AssignmentResponse is already in trainer_handler.go (or a shared DTO file)


// --- Handler Methods for Client ---

// GetMyTrainingPlans godoc
// @Summary Get my assigned training plans
// @Description Retrieves training plans assigned to the authenticated client.
// @Tags Client
// @Produce json
// @Security BearerAuth
// @Success 200 {array} TrainingPlanResponse "List of training plans"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /client/plans [get]
func (h *ClientHandler) GetMyTrainingPlans(c *gin.Context) {
	clientIDStr, err := getUserIDFromContext(c) // From JWT
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify client.")
		return
	}
	clientID, err := primitive.ObjectIDFromHex(clientIDStr)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid client ID format in token.")
		return
	}

	plans, err := h.clientService.GetMyActiveTrainingPlans(c.Request.Context(), clientID)
	if err != nil {
        // Handle specific errors from service if defined, e.g., ErrClientNotFound
        // log.Printf("Error fetching client's plans: %v", err)
		abortWithError(c, http.StatusInternalServerError, "Failed to retrieve training plans.")
		return
	}
    if plans == nil { // Service might return nil for no plans
        c.JSON(http.StatusOK, []TrainingPlanResponse{})
        return
    }
	c.JSON(http.StatusOK, MapTrainingPlansToResponse(plans)) // Reuse mapper from trainer_handler
}


// GetWorkoutsForMyPlan godoc
// @Summary Get workouts for one of my training plans
// @Description Retrieves workouts for a specific training plan assigned to the authenticated client.
// @Tags Client
// @Produce json
// @Security BearerAuth
// @Param planId path string true "Training Plan's ObjectID Hex"
// @Success 200 {array} WorkoutResponse "List of workouts"
// @Failure 400 {object} gin.H "Invalid plan ID format"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (plan not assigned to this client)"
// @Failure 404 {object} gin.H "Training Plan not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /client/plans/{planId}/workouts [get]
func (h *ClientHandler) GetWorkoutsForMyPlan(c *gin.Context) {
	clientIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify client.")
		return
	}
	clientID, _ := primitive.ObjectIDFromHex(clientIDStr) // Ignore error as it's checked before

	planIDHex := c.Param("planId")
	planID, err := primitive.ObjectIDFromHex(planIDHex)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid plan ID format.")
		return
	}

	workouts, err := h.clientService.GetWorkoutsForMyPlan(c.Request.Context(), clientID, planID)
	if err != nil {
		if errors.Is(err, service.ErrTrainingPlanNotFound) {
			abortWithError(c, http.StatusNotFound, err.Error())
        } else if errors.Is(err, service.ErrPlanNotAssignedToClient) {
            abortWithError(c, http.StatusForbidden, err.Error())
		} else {
			abortWithError(c, http.StatusInternalServerError, "Failed to retrieve workouts.")
		}
		return
	}
    if workouts == nil {
        c.JSON(http.StatusOK, []WorkoutResponse{})
        return
    }
	c.JSON(http.StatusOK, MapWorkoutsToResponse(workouts)) // Reuse mapper
}


// GetAssignmentsForMyWorkout godoc
// @Summary Get assignments for one of my workouts
// @Description Retrieves exercise assignments for a specific workout within a plan assigned to the authenticated client.
// @Tags Client
// @Produce json
// @Security BearerAuth
// @Param workoutId path string true "Workout's ObjectID Hex"
// @Success 200 {array} AssignmentResponse "List of assignments"
// @Failure 400 {object} gin.H "Invalid workout ID format"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (workout not assigned to this client)"
// @Failure 404 {object} gin.H "Workout not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /client/workouts/{workoutId}/assignments [get]
func (h *ClientHandler) GetAssignmentsForMyWorkout(c *gin.Context) {
	clientIDStr, err := getUserIDFromContext(c)
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "Unable to identify client.")
		return
	}
	clientID, _ := primitive.ObjectIDFromHex(clientIDStr)

	workoutIDHex := c.Param("workoutId")
	workoutID, err := primitive.ObjectIDFromHex(workoutIDHex)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid workout ID format.")
		return
	}

	assignments, err := h.clientService.GetAssignmentsForMyWorkout(c.Request.Context(), clientID, workoutID)
	if err != nil {
		if errors.Is(err, service.ErrWorkoutNotFound) {
			abortWithError(c, http.StatusNotFound, err.Error())
        } else if errors.Is(err, service.ErrWorkoutNotBelongToPlan) { // Or a more generic auth error
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
	c.JSON(http.StatusOK, MapAssignmentsToResponse(assignments)) // Reuse mapper
}

// --- DTO for Updating Assignment Status ---
type UpdateAssignmentStatusRequest struct {
	Status string `json:"status" binding:"required"` // Expecting "completed", "submitted", etc.
}


// --- Handler Methods for Client ---
// ... (GetMyTrainingPlans, GetWorkoutsForMyPlan, GetAssignmentsForMyWorkout) ...


// UpdateMyAssignmentStatus godoc
// @Summary Update the status of one of my assignments
// @Description Allows a client to update the status of their exercise assignment (e.g., mark as completed).
// @Tags Client Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assignmentId path string true "Assignment's ObjectID Hex"
// @Param statusRequest body UpdateAssignmentStatusRequest true "New status"
// @Success 200 {object} AssignmentResponse "Assignment status updated successfully"
// @Failure 400 {object} gin.H "Invalid input (validation error, invalid ID, invalid status)"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (assignment not for this client, or invalid status transition)"
// @Failure 404 {object} gin.H "Assignment not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /client/assignments/{assignmentId}/status [patch]
func (h *ClientHandler) UpdateMyAssignmentStatus(c *gin.Context) {
	clientIDStr, err := getUserIDFromContext(c)
	if err != nil {
			abortWithError(c, http.StatusUnauthorized, "Unable to identify client.")
			return
	}
	clientID, _ := primitive.ObjectIDFromHex(clientIDStr) // Assume valid if token is good

	assignmentIDHex := c.Param("assignmentId")
	assignmentID, err := primitive.ObjectIDFromHex(assignmentIDHex)
	if err != nil {
			abortWithError(c, http.StatusBadRequest, "Invalid assignment ID format.")
			return
	}

	var req UpdateAssignmentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
			abortWithError(c, http.StatusBadRequest, "Validation error: "+err.Error())
			return
	}

	// Convert string status from request to domain.AssignmentStatus
	// Add validation here for allowed status strings from client
	newStatus := domain.AssignmentStatus(req.Status)
	if newStatus != domain.StatusCompleted { // Example: Client can only mark as 'completed'
			// You might want more sophisticated status transition logic in the service
			abortWithError(c, http.StatusBadRequest, "Invalid status value provided by client.")
			return
	}


	updatedAssignment, err := h.clientService.UpdateMyAssignmentStatus(c.Request.Context(), clientID, assignmentID, newStatus)
	if err != nil {
			if errors.Is(err, service.ErrAssignmentNotFound) {
					abortWithError(c, http.StatusNotFound, err.Error())
			} else if errors.Is(err, service.ErrAssignmentNotBelongToClient) || errors.Is(err, service.ErrInvalidAssignmentStatusUpdate) {
					abortWithError(c, http.StatusForbidden, err.Error())
			} else if errors.Is(err, service.ErrWorkoutNotFound) { // If service propagates this
					 abortWithError(c, http.StatusNotFound, "Associated workout not found, cannot update status.")
			} else {
					// log.Printf("Error updating assignment status: %v", err)
					abortWithError(c, http.StatusInternalServerError, "Failed to update assignment status.")
			}
			return
	}

	c.JSON(http.StatusOK, MapAssignmentToResponse(updatedAssignment)) // Reuse existing mapper
}

type RequestUploadURLRequest struct {
	ContentType string `json:"contentType" binding:"required"`
}

// UploadURLResponse is already defined (or should be in a shared DTO place)
// type UploadURLResponse struct {
// 	UploadURL string `json:"uploadUrl"`
// 	ObjectKey string `json:"objectKey"`
// }

type ConfirmUploadRequest struct {
	ObjectKey   string `json:"objectKey" binding:"required"`
	FileName    string `json:"fileName" binding:"required"`
	FileSize    int64  `json:"fileSize" binding:"required,min=1"`
	ContentType string `json:"contentType" binding:"required"`
}

// RequestUploadURLForAssignment godoc
// @Summary Request a pre-signed URL to upload a video for an assignment
// @Description Client requests a temporary URL to directly upload their video proof to S3.
// @Tags Client Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assignmentId path string true "Assignment's ObjectID Hex"
// @Param uploadRequest body RequestUploadURLRequest true "Upload content type"
// @Success 200 {object} UploadURLResponse "Pre-signed URL and object key"
// @Failure 400 {object} gin.H "Invalid input"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (assignment not for this client, or upload not allowed for status)"
// @Failure 404 {object} gin.H "Assignment not found"
// @Failure 500 {object} gin.H "Internal Server Error (e.g., S3 error)"
// @Router /client/assignments/{assignmentId}/upload-url [post]
func (h *ClientHandler) RequestUploadURLForAssignment(c *gin.Context) {
	clientIDStr, err := getUserIDFromContext(c)
	if err != nil { abortWithError(c, http.StatusUnauthorized, "Unauthorized."); return }
	clientID, _ := primitive.ObjectIDFromHex(clientIDStr)

	assignmentIDHex := c.Param("assignmentId")
	assignmentID, err := primitive.ObjectIDFromHex(assignmentIDHex)
	if err != nil { abortWithError(c, http.StatusBadRequest, "Invalid assignment ID."); return }

	var req RequestUploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Call clientService method (which we created earlier)
	// Make sure the service.UploadURLResponse matches the DTO api.UploadURLResponse
	// Or map it here.
	serviceResponse, err := h.clientService.RequestUploadURL(c.Request.Context(), clientID, assignmentID, req.ContentType)
	if err != nil {
		log.Printf("Service Error in RequestUploadURLForAssignment: %v", err)
		// Map service errors (ErrAssignmentNotFound, ErrAssignmentNotBelongToClient, ErrUploadNotAllowed, ErrUploadURLError)
		if errors.Is(err, service.ErrAssignmentNotFound) {
			abortWithError(c, http.StatusNotFound, err.Error())
        } else if errors.Is(err, service.ErrAssignmentNotBelongToClient) || errors.Is(err, service.ErrUploadNotAllowed) {
            abortWithError(c, http.StatusForbidden, err.Error())
        } else if errors.Is(err, service.ErrUploadURLError) || errors.Is(err, service.ErrWorkoutNotFound) { // Workout check is now in service
             abortWithError(c, http.StatusInternalServerError, err.Error())
		} else {
			abortWithError(c, http.StatusInternalServerError, "Failed to get upload URL.")
		}
		return
	}
    // Assuming service.UploadURLResponse is compatible with api.UploadURLResponse
	c.JSON(http.StatusOK, serviceResponse)
}


// ConfirmUploadForAssignment godoc
// @Summary Confirm video upload for an assignment
// @Description Client informs the backend that the S3 upload is complete. Backend updates assignment status.
// @Tags Client Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assignmentId path string true "Assignment's ObjectID Hex"
// @Param confirmRequest body ConfirmUploadRequest true "Upload confirmation details"
// @Success 200 {object} AssignmentResponse "Assignment updated successfully"
// @Failure 400 {object} gin.H "Invalid input"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (assignment not for this client)"
// @Failure 404 {object} gin.H "Assignment not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /client/assignments/{assignmentId}/upload-confirm [post]
func (h *ClientHandler) ConfirmUploadForAssignment(c *gin.Context) {
	clientIDStr, err := getUserIDFromContext(c)
	if err != nil { abortWithError(c, http.StatusUnauthorized, "Unauthorized."); return }
	clientID, _ := primitive.ObjectIDFromHex(clientIDStr)

	assignmentIDHex := c.Param("assignmentId")
	assignmentID, err := primitive.ObjectIDFromHex(assignmentIDHex)
	if err != nil { abortWithError(c, http.StatusBadRequest, "Invalid assignment ID."); return }

	var req ConfirmUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	updatedAssignment, err := h.clientService.ConfirmUpload(
		c.Request.Context(),
		clientID,
		assignmentID,
		req.ObjectKey,
		req.FileName,
		req.FileSize,
		req.ContentType,
	)
	if err != nil {
		// Map service errors
        if errors.Is(err, service.ErrAssignmentNotFound) {
			abortWithError(c, http.StatusNotFound, err.Error())
        } else if errors.Is(err, service.ErrAssignmentNotBelongToClient) {
            abortWithError(c, http.StatusForbidden, err.Error())
        } else if errors.Is(err, service.ErrUploadConfirmationFailed) || errors.Is(err, service.ErrWorkoutNotFound) {
             abortWithError(c, http.StatusInternalServerError, err.Error())
		} else {
			abortWithError(c, http.StatusInternalServerError, "Failed to confirm upload.")
		}
		return
	}
	c.JSON(http.StatusOK, MapAssignmentToResponse(updatedAssignment)) // Use existing mapper
}

// --- DTO for Logging Performance ---
type LogPerformanceRequest struct {
	AchievedSets          *int    `json:"achievedSets" binding:"omitempty,min=0"` // min=0 allows logging 0 sets
	AchievedReps          *string `json:"achievedReps" binding:"omitempty"`
	AchievedWeight        *string `json:"achievedWeight" binding:"omitempty"`
	AchievedDuration      *string `json:"achievedDuration" binding:"omitempty"`
	ClientPerformanceNotes *string `json:"clientPerformanceNotes" binding:"omitempty"` // Changed to *string to match domain
	// Status string `json:"status,omitempty"` // Optionally allow client to update status while logging
}

// LogPerformanceForMyAssignment godoc
// @Summary Log performance for an assignment
// @Description Allows a client to log their achieved sets, reps, weight, etc., for an assignment.
// @Tags Client Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assignmentId path string true "Assignment's ObjectID Hex"
// @Param performanceRequest body LogPerformanceRequest true "Achieved performance data"
// @Success 200 {object} AssignmentResponse "Performance logged successfully"
// @Failure 400 {object} gin.H "Invalid input"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 403 {object} gin.H "Forbidden (assignment not for this client)"
// @Failure 404 {object} gin.H "Assignment not found"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /client/assignments/{assignmentId}/performance [patch]
func (h *ClientHandler) LogPerformanceForMyAssignment(c *gin.Context) {
	clientIDStr, err := getUserIDFromContext(c)
	if err != nil { abortWithError(c, http.StatusUnauthorized, "Unauthorized."); return }
	clientID, _ := primitive.ObjectIDFromHex(clientIDStr)

	assignmentIDHex := c.Param("assignmentId")
	assignmentID, err := primitive.ObjectIDFromHex(assignmentIDHex)
	if err != nil { abortWithError(c, http.StatusBadRequest, "Invalid assignment ID."); return }

	var req LogPerformanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
			abortWithError(c, http.StatusBadRequest, "Validation error: "+err.Error())
			return
	}

	// Construct domain.Assignment with only the fields being updated by performance logging
	performanceData := domain.Assignment{
			AchievedSets:          req.AchievedSets,
			AchievedReps:          req.AchievedReps,
			AchievedWeight:        req.AchievedWeight,
			AchievedDuration:      req.AchievedDuration,
			ClientPerformanceNotes: req.ClientPerformanceNotes,
			// If status is part of this request:
			// Status: domain.AssignmentStatus(req.Status),
	}

	updatedAssignment, err := h.clientService.LogPerformanceForMyAssignment(c.Request.Context(), clientID, assignmentID, performanceData)
	if err != nil {
			// Map service errors appropriately
			if errors.Is(err, service.ErrAssignmentNotFound) || errors.Is(err, service.ErrWorkoutNotFound) {
					 abortWithError(c, http.StatusNotFound, err.Error())
			} else if errors.Is(err, service.ErrAssignmentNotBelongToClient) {
					 abortWithError(c, http.StatusForbidden, err.Error())
			} else {
					 abortWithError(c, http.StatusInternalServerError, "Failed to log performance.")
			}
			return
	}
	c.JSON(http.StatusOK, MapAssignmentToResponse(updatedAssignment)) // Reuse existing mapper
}

// GetMyCurrentWorkouts godoc
// @Summary Get my current workout(s) for today
// @Description Retrieves the workout(s) scheduled for the authenticated client for the current day.
// @Tags Client Workouts
// @Produce json
// @Security BearerAuth
// @Success 200 {array} WorkoutResponse "List of current workout(s) for today (can be empty)"
// @Failure 401 {object} gin.H "Unauthorized"
// @Failure 500 {object} gin.H "Internal Server Error"
// @Router /client/workouts/today [get]
func (h *ClientHandler) GetMyCurrentWorkouts(c *gin.Context) {
	clientIDStr, err := getUserIDFromContext(c)
	if err != nil { abortWithError(c, http.StatusUnauthorized, "Unauthorized."); return }
	clientID, _ := primitive.ObjectIDFromHex(clientIDStr)

	// Use current server date. Could also allow a 'date' query param for testing.
	today := time.Now().UTC() // Or use a specific timezone relevant to your users

	workouts, err := h.clientService.GetMyCurrentWorkouts(c.Request.Context(), clientID, today)
	if err != nil {
			// Handle specific errors from service if needed, e.g., client not found
			// log.Printf("Error getting current workouts for client %s: %v", clientIDStr, err)
			abortWithError(c, http.StatusInternalServerError, "Failed to retrieve current workout(s).")
			return
	}

	// Service returns empty slice if no workouts for today, not an error.
	if workouts == nil { // Should be an empty slice, not nil, from service
			 c.JSON(http.StatusOK, []WorkoutResponse{})
			 return
	}
	c.JSON(http.StatusOK, MapWorkoutsToResponse(workouts)) // Reuse existing mapper
}