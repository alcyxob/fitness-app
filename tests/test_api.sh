#!/bin/bash

# API Test Script for Fitness App (including Client View Endpoints)

# --- Configuration ---
BASE_URL="http://localhost:8080/api/v1"
TIMESTAMP=$(date +%s) # Unique IDs for this test run

# Trainer
TRAINER_NAME="Test Trainer $TIMESTAMP"
TRAINER_EMAIL="trainer.$TIMESTAMP@example.com"
TRAINER_PASS="password123"

# Client
CLIENT_NAME="Test Client $TIMESTAMP"
CLIENT_EMAIL="client.$TIMESTAMP@example.com"
CLIENT_PASS="password456"

# Variables to store dynamic data
TRAINER_TOKEN=""
TRAINER_ID=""
CLIENT_TOKEN=""
CLIENT_ID=""
EXERCISE_ID_ONE=""
TRAINING_PLAN_ID_ONE=""
WORKOUT_ID_ONE=""
ASSIGNMENT_ID_ONE=""


# --- Helper Functions ---
check_jq() {
  if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed. Please install jq."
    exit 1
  fi
}

expect_status() {
  local expected_code="$1"
  shift
  local response_and_code=$(curl -s -w "\n%{http_code}" "$@") # Capture body and code
  local http_code=$(echo "$response_and_code" | tail -n1)
  local response_body=$(echo "$response_and_code" | sed '$d')

  if [[ "$http_code" -ne "$expected_code" ]]; then
    echo "❌ FAILED: Expected status $expected_code, but got $http_code for request: curl $*"
    echo "   Response Body: $response_body"
    return 1
  else
    echo "✅ PASSED: Received expected status $expected_code."
    # Optionally echo body for successful 200/201 if needed for debugging
    # if [[ "$http_code" -eq 200 || "$http_code" -eq 201 ]]; then
    #   echo "   Response Body: $response_body"
    # fi
    return 0
  fi
}

# Function to make request and store response body
# Usage: store_response VARIABLE_NAME <curl_args...>
store_response() {
  local var_name="$1"
  shift
  local response_and_code=$(curl -s -w "\n%{http_code}" "$@")
  local http_code=$(echo "$response_and_code" | tail -n1)
  local response_body=$(echo "$response_and_code" | sed '$d')

  if [[ "$http_code" -ne 200 && "$http_code" -ne 201 ]]; then # Expect 200 or 201 for success
    echo "❌ FAILED (store_response): Expected status 200/201, but got $http_code for request: curl $*"
    echo "   Response Body: $response_body"
    eval "$var_name=''" # Clear variable on failure
    return 1
  else
    echo "✅ PASSED (store_response): Received status $http_code."
    eval "$var_name='$response_body'" # Store response body
    return 0
  fi
}


# --- Test Execution ---
check_jq

echo "🚀 Starting API Tests for Fitness App ($BASE_URL)..."
echo "--------------------------------------------------"

# === Phase 1: Setup - Users ===
echo "Phase 1: User Registration & Login"

# 1. Register Trainer
echo "🧪 Test 1.1: Registering Trainer ($TRAINER_EMAIL)..."
register_payload_trainer=$(cat <<EOF
{ "name": "$TRAINER_NAME", "email": "$TRAINER_EMAIL", "password": "$TRAINER_PASS", "role": "trainer" }
EOF
)
expect_status 201 -X POST -H "Content-Type: application/json" -d "$register_payload_trainer" "$BASE_URL/auth/register" || exit 1
echo "---"

# 2. Register Client
echo "🧪 Test 1.2: Registering Client ($CLIENT_EMAIL)..."
register_payload_client=$(cat <<EOF
{ "name": "$CLIENT_NAME", "email": "$CLIENT_EMAIL", "password": "$CLIENT_PASS", "role": "client" }
EOF
)
expect_status 201 -X POST -H "Content-Type: application/json" -d "$register_payload_client" "$BASE_URL/auth/register" || exit 1
echo "---"

# 3. Login Trainer
echo "🧪 Test 1.3: Logging in as Trainer..."
login_payload_trainer=$(cat <<EOF
{ "email": "$TRAINER_EMAIL", "password": "$TRAINER_PASS" }
EOF
)
store_response LOGIN_RESPONSE_TRAINER -X POST -H "Content-Type: application/json" -d "$login_payload_trainer" "$BASE_URL/auth/login" || exit 1
TRAINER_TOKEN=$(echo "$LOGIN_RESPONSE_TRAINER" | jq -r '.token // empty')
TRAINER_ID=$(echo "$LOGIN_RESPONSE_TRAINER" | jq -r '.user.id // empty')
if [[ -z "$TRAINER_TOKEN" ]]; then echo "❌ FAILED: Could not get Trainer token."; exit 1; fi
echo "   Trainer Token: HIDDEN, Trainer ID: $TRAINER_ID"
echo "--------------------------------------------------"


# === Phase 2: Trainer Sets Up Client and Program ===
echo "Phase 2: Trainer Setup (Add Client, Exercise, Plan, Workout, Assignment)"

# 4. Trainer Adds Client
echo "🧪 Test 2.1: Trainer adds Client ($CLIENT_EMAIL)..."
add_client_payload=$(cat <<EOF
{ "clientEmail": "$CLIENT_EMAIL" }
EOF
)
expect_status 200 -X POST -H "Authorization: Bearer $TRAINER_TOKEN" -H "Content-Type: application/json" -d "$add_client_payload" "$BASE_URL/trainer/clients" || exit 1
echo "---"

# 5. Trainer Creates an Exercise
echo "🧪 Test 2.2: Trainer creates an Exercise..."
exercise_payload=$(cat <<EOF
{
  "name": "Test Push-ups $TIMESTAMP",
  "description": "Classic upper body strength.",
  "muscleGroup": "Chest, Triceps, Shoulders",
  "executionTechnic": "Keep body straight, lower till chest nears floor.",
  "applicability": "Any",
  "difficulty": "Medium"
}
EOF
)
store_response CREATE_EXERCISE_RESPONSE -X POST -H "Authorization: Bearer $TRAINER_TOKEN" -H "Content-Type: application/json" -d "$exercise_payload" "$BASE_URL/exercises" || exit 1
EXERCISE_ID_ONE=$(echo "$CREATE_EXERCISE_RESPONSE" | jq -r '.id // empty')
if [[ -z "$EXERCISE_ID_ONE" ]]; then echo "❌ FAILED: Could not get Exercise ID."; exit 1; fi
echo "   Created Exercise ID: $EXERCISE_ID_ONE"
echo "---"

# 6. Trainer Creates a Training Plan for the Client
echo "🧪 Test 2.3: Trainer creates a Training Plan for Client..."
# First, get Client ID from the managed clients list (or assume it's CLIENT_ID if we logged them in)
# For simplicity, let's fetch clients again to get the ID if needed, or use the ID from client registration if we had it
# Assume the client added was the one we registered (CLIENT_EMAIL)
# Let's get client ID from login response when CLIENT logs in, or if trainer/clients returns full user object
# For now, let's get the actual client ID by fetching the specific client.
# This is a bit more robust than assuming based on email.
# A better API might return the client ID upon successful addition to trainer.
temp_client_list_resp=$(curl -s -X GET -H "Authorization: Bearer $TRAINER_TOKEN" "$BASE_URL/trainer/clients")
CLIENT_ID=$(echo "$temp_client_list_resp" | jq -r --arg email "$CLIENT_EMAIL" '.[] | select(.email == $email) | .id // empty')
if [[ -z "$CLIENT_ID" ]]; then echo "❌ FAILED: Could not find Client ID for $CLIENT_EMAIL in trainer's list."; exit 1; fi
echo "   Found Client ID for assignment: $CLIENT_ID"

plan_payload=$(cat <<EOF
{
  "name": "Client Strength Plan $TIMESTAMP",
  "description": "4-week beginner strength program.",
  "isActive": true
}
EOF
)
store_response CREATE_PLAN_RESPONSE -X POST \
  -H "Authorization: Bearer $TRAINER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$plan_payload" \
  "$BASE_URL/trainer/clients/$CLIENT_ID/plans" || exit 1
TRAINING_PLAN_ID_ONE=$(echo "$CREATE_PLAN_RESPONSE" | jq -r '.id // empty')
if [[ -z "$TRAINING_PLAN_ID_ONE" ]]; then echo "❌ FAILED: Could not get Training Plan ID."; exit 1; fi
echo "   Created Training Plan ID: $TRAINING_PLAN_ID_ONE"
echo "---"

# 7. Trainer Creates a Workout in that Plan
echo "🧪 Test 2.4: Trainer creates a Workout in the Plan..."
workout_payload=$(cat <<EOF
{
  "name": "Full Body A - $TIMESTAMP",
  "sequence": 0,
  "notes": "Focus on form."
}
EOF
)
store_response CREATE_WORKOUT_RESPONSE -X POST \
  -H "Authorization: Bearer $TRAINER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$workout_payload" \
  "$BASE_URL/trainer/plans/$TRAINING_PLAN_ID_ONE/workouts" || exit 1
WORKOUT_ID_ONE=$(echo "$CREATE_WORKOUT_RESPONSE" | jq -r '.id // empty')
if [[ -z "$WORKOUT_ID_ONE" ]]; then echo "❌ FAILED: Could not get Workout ID."; exit 1; fi
echo "   Created Workout ID: $WORKOUT_ID_ONE"
echo "---"

# 8. Trainer Assigns Exercise to Workout
echo "🧪 Test 2.5: Trainer assigns Exercise to Workout..."
assign_payload=$(cat <<EOF
{
  "exerciseId": "$EXERCISE_ID_ONE",
  "sets": 3,
  "reps": "8-12",
  "rest": "60s",
  "sequence": 0,
  "trainerNotes": "First exercise, warm up well."
}
EOF
)
store_response CREATE_ASSIGNMENT_RESPONSE -X POST \
  -H "Authorization: Bearer $TRAINER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$assign_payload" \
  "$BASE_URL/trainer/workouts/$WORKOUT_ID_ONE/exercises" || exit 1
ASSIGNMENT_ID_ONE=$(echo "$CREATE_ASSIGNMENT_RESPONSE" | jq -r '.id // empty')
if [[ -z "$ASSIGNMENT_ID_ONE" ]]; then echo "❌ FAILED: Could not get Assignment ID."; exit 1; fi
echo "   Created Assignment ID: $ASSIGNMENT_ID_ONE"
echo "--------------------------------------------------"


# === Phase 3: Client Interaction ===
echo "Phase 3: Client Interaction (Login, View, Mark Complete, Upload Video)"
# 3.1 Login Client
echo "🧪 Test 3.1: Logging in as Client ($CLIENT_EMAIL)..."
login_payload_client=$(jq -n --arg email "$CLIENT_EMAIL" --arg pass "$CLIENT_PASS" '{email: $email, password: $pass}')
store_response LOGIN_RESPONSE_CLIENT -X POST -H "Content-Type: application/json" -d "$login_payload_client" "$BASE_URL/auth/login" || exit 1
CLIENT_TOKEN=$(echo "$LOGIN_RESPONSE_CLIENT" | jq -r '.token // empty')
if [[ -z "$CLIENT_TOKEN" ]]; then echo "❌ FAILED: Could not get Client token."; exit 1; fi
echo "   Client Token: SET"
# 3.2 Client Fetches Plans, Workouts, Assignments (Simplified checks from before)
echo "🧪 Test 3.2: Client fetches their program structure..."
expect_status 200 -X GET -H "Authorization: Bearer $CLIENT_TOKEN" "$BASE_URL/client/plans" || exit 1
expect_status 200 -X GET -H "Authorization: Bearer $CLIENT_TOKEN" "$BASE_URL/client/plans/$TRAINING_PLAN_ID_ONE/workouts" || exit 1
expect_status 200 -X GET -H "Authorization: Bearer $CLIENT_TOKEN" "$BASE_URL/client/workouts/$WORKOUT_ID_ONE/assignments" || exit 1
echo "---"

# 3.3 Client Marks Assignment as Complete
echo "🧪 Test 3.3: Client marks Assignment $ASSIGNMENT_ID_ONE as 'completed'..."
status_payload=$(jq -n '{status: "completed"}')
store_response UPDATE_STATUS_RESPONSE -X PATCH \
  -H "Authorization: Bearer $CLIENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$status_payload" \
  "$BASE_URL/client/assignments/$ASSIGNMENT_ID_ONE/status" || exit 1
updated_status=$(echo "$UPDATE_STATUS_RESPONSE" | jq -r '.status // empty')
if [[ "$updated_status" != "completed" ]]; then echo "❌ FAILED: Status not updated to completed."; exit 1; fi
echo "   Assignment status updated to: $updated_status"
echo "---"

# 3.4 Client Requests S3 Upload URL for the Assignment
echo "🧪 Test 3.4: Client requests S3 Upload URL for Assignment $ASSIGNMENT_ID_ONE..."
# For this test, we need a dummy video file. Create one if it doesn't exist.
DUMMY_VIDEO_FILE="dummy_video_test.mp4"
DUMMY_VIDEO_CONTENT_TYPE="video/mp4"
if [ ! -f "$DUMMY_VIDEO_FILE" ]; then
  echo "Creating dummy video file: $DUMMY_VIDEO_FILE"
  dd if=/dev/zero of="$DUMMY_VIDEO_FILE" bs=1024 count=10 # Create a 10KB dummy file
fi
DUMMY_VIDEO_SIZE=$(stat -f%z "$DUMMY_VIDEO_FILE") # macOS syntax for stat; for Linux use: stat -c%s

upload_url_payload=$(jq -n --arg type "$DUMMY_VIDEO_CONTENT_TYPE" '{contentType: $type}')
store_response UPLOAD_URL_RESPONSE -X POST \
  -H "Authorization: Bearer $CLIENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$upload_url_payload" \
  "$BASE_URL/client/assignments/$ASSIGNMENT_ID_ONE/upload-url" || exit 1

S3_UPLOAD_URL=$(echo "$UPLOAD_URL_RESPONSE" | jq -r '.uploadUrl // empty')
VIDEO_OBJECT_KEY=$(echo "$UPLOAD_URL_RESPONSE" | jq -r '.objectKey // empty')

if [[ -z "$S3_UPLOAD_URL" || -z "$VIDEO_OBJECT_KEY" ]]; then echo "❌ FAILED: Could not get S3 Upload URL or Object Key."; exit 1; fi
echo "   S3 Upload URL: RECEIVED (long URL)"
echo "   S3 Object Key: $VIDEO_OBJECT_KEY"
echo "---"

# 3.5 Client Uploads Dummy Video to S3
echo "🧪 Test 3.5: Client uploads dummy video to S3 URL..."
# We expect MinIO to be running locally without SSL for this test.
# The S3_UPLOAD_URL should point to localhost:9000 (or your Mac's IP if testing device against Mac)
# Ensure your Go backend's S3_PUBLIC_ENDPOINT is configured to generate this localhost URL.
s3_upload_http_code=$(curl -s -o /dev/null -w "%{http_code}" \
  -X PUT \
  -H "Content-Type: $DUMMY_VIDEO_CONTENT_TYPE" \
  --data-binary @"$DUMMY_VIDEO_FILE" \
  "$S3_UPLOAD_URL")

if [[ "$s3_upload_http_code" -ne 200 ]]; then
  echo "❌ FAILED: S3 upload failed. Expected 200, Got $s3_upload_http_code."
  echo "   Attempted to upload to: $S3_UPLOAD_URL"
  exit 1
else
  echo "✅ PASSED: Dummy video uploaded to S3. Status $s3_upload_http_code."
fi
echo "---"

# 3.6 Client Confirms Upload with Backend
echo "🧪 Test 3.6: Client confirms video upload with backend..."
confirm_payload=$(jq -n \
  --arg key "$VIDEO_OBJECT_KEY" \
  --arg name "$DUMMY_VIDEO_FILE" \
  --argjson size "$DUMMY_VIDEO_SIZE" \
  --arg type "$DUMMY_VIDEO_CONTENT_TYPE" \
  '{objectKey: $key, fileName: $name, fileSize: $size, contentType: $type}')

store_response CONFIRM_UPLOAD_RESPONSE -X POST \
  -H "Authorization: Bearer $CLIENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$confirm_payload" \
  "$BASE_URL/client/assignments/$ASSIGNMENT_ID_ONE/upload-confirm" || exit 1

confirmed_status=$(echo "$CONFIRM_UPLOAD_RESPONSE" | jq -r '.status // empty')
confirmed_upload_id=$(echo "$CONFIRM_UPLOAD_RESPONSE" | jq -r '.uploadId // empty')

if [[ "$confirmed_status" != "submitted" || -z "$confirmed_upload_id" ]]; then
  echo "❌ FAILED: Upload confirmation failed or status not 'submitted' or uploadId missing."
  echo "   Response: $CONFIRM_UPLOAD_RESPONSE"
  exit 1
fi
echo "   Upload confirmed. Assignment status: $confirmed_status, Upload ID: $confirmed_upload_id"
echo "--------------------------------------------------"


# === Phase 4: Trainer Views Client Submission ===
echo "Phase 4: Trainer Views Submission"

# 4.1 Trainer Fetches Video Download URL
echo "🧪 Test 4.1: Trainer fetches video download URL for Assignment $ASSIGNMENT_ID_ONE..."
store_response TRAINER_VIDEO_URL_RESPONSE -X GET \
  -H "Authorization: Bearer $TRAINER_TOKEN" \
  "$BASE_URL/trainer/assignments/$ASSIGNMENT_ID_ONE/video-download-url" || exit 1

VIDEO_DOWNLOAD_URL=$(echo "$TRAINER_VIDEO_URL_RESPONSE" | jq -r '.downloadUrl // empty')
if [[ -z "$VIDEO_DOWNLOAD_URL" ]]; then echo "❌ FAILED: Could not get video download URL."; exit 1; fi
echo "   Video Download URL: RECEIVED (long URL)"
echo "---"

# 4.2 (Informational) Trainer could now use this URL to view the video
echo "   ℹ️ Trainer can now use this URL to download/view the video: $VIDEO_DOWNLOAD_URL"
echo "      (This script won't download it, just verifies URL generation)"
echo "--------------------------------------------------"

echo "🏁 All API Tests Finished Successfully."

# Cleanup dummy file
if [ -f "$DUMMY_VIDEO_FILE" ]; then
  rm "$DUMMY_VIDEO_FILE"
  echo "🧹 Cleaned up $DUMMY_VIDEO_FILE"
fi