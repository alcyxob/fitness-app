#!/bin/bash

# Simple API Test Script for Fitness App

# --- Configuration ---
BASE_URL="http://localhost:8080/api/v1"
TRAINER_EMAIL="test.trainer.$(date +%s)@example.com" # Use timestamp for uniqueness
TRAINER_PASS="password123"
CLIENT_EMAIL="test.client.$(date +%s)@example.com" # Use timestamp for uniqueness
CLIENT_PASS="password456"

# Variables to store tokens
TRAINER_TOKEN=""
CLIENT_TOKEN=""

# --- Helper Functions ---

# Function to check if jq is installed
check_jq() {
  if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed. Please install jq (e.g., 'brew install jq' or 'sudo apt install jq')."
    exit 1
  fi
}

# Function to make requests and check status code
# Usage: expect_status <expected_code> <curl_args...>
expect_status() {
  local expected_code="$1"
  shift # Remove expected_code from arguments
  local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$@")
  if [[ "$http_code" -ne "$expected_code" ]]; then
    echo "❌ FAILED: Expected status $expected_code, but got $http_code for request: curl $*"
    # Optionally print response body on error for debugging:
    # echo "Response body:"
    # curl -s "$@"
    return 1 # Failure
  else
    echo "✅ PASSED: Received expected status $expected_code."
    return 0 # Success
  fi
}

# --- Test Execution ---

check_jq # Ensure jq is available

echo "🚀 Starting API Tests for Fitness App ($BASE_URL)..."
echo "--------------------------------------------------"

# 1. Ping Test (Public)
echo "🧪 Test 1: Checking API health (/ping)..."
expect_status 200 "$BASE_URL/ping"
echo "--------------------------------------------------"

# 2. Register Trainer (Public)
echo "🧪 Test 2: Registering a new Trainer..."
echo "   Email: $TRAINER_EMAIL"
register_payload_trainer=$(cat <<EOF
{
  "name": "Test Trainer",
  "email": "$TRAINER_EMAIL",
  "password": "$TRAINER_PASS",
  "role": "trainer"
}
EOF
)
expect_status 201 -X POST -H "Content-Type: application/json" -d "$register_payload_trainer" "$BASE_URL/auth/register"
echo "--------------------------------------------------"

# 3. Register Client (Public)
echo "🧪 Test 3: Registering a new Client..."
echo "   Email: $CLIENT_EMAIL"
register_payload_client=$(cat <<EOF
{
  "name": "Test Client",
  "email": "$CLIENT_EMAIL",
  "password": "$CLIENT_PASS",
  "role": "client"
}
EOF
)
expect_status 201 -X POST -H "Content-Type: application/json" -d "$register_payload_client" "$BASE_URL/auth/register"
echo "--------------------------------------------------"

# 4. Register Duplicate Trainer (Public - Expect Conflict)
echo "🧪 Test 4: Attempting to register duplicate Trainer (expect 409)..."
expect_status 409 -X POST -H "Content-Type: application/json" -d "$register_payload_trainer" "$BASE_URL/auth/register"
echo "--------------------------------------------------"

# 5. Login Trainer (Public)
echo "🧪 Test 5: Logging in as Trainer..."
login_payload_trainer=$(cat <<EOF
{
  "email": "$TRAINER_EMAIL",
  "password": "$TRAINER_PASS"
}
EOF
)
login_response_trainer=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "$login_payload_trainer" \
  "$BASE_URL/auth/login")

# Extract token using jq
TRAINER_TOKEN=$(echo "$login_response_trainer" | jq -r '.token // empty')

if [[ -z "$TRAINER_TOKEN" ]]; then
  echo "❌ FAILED: Could not extract Trainer token from login response."
  echo "Response: $login_response_trainer"
else
  echo "✅ PASSED: Trainer login successful. Token stored."
  # Optionally print user details: echo "$login_response_trainer" | jq '.user'
fi
echo "--------------------------------------------------"

# 6. Login Client (Public)
echo "🧪 Test 6: Logging in as Client..."
login_payload_client=$(cat <<EOF
{
  "email": "$CLIENT_EMAIL",
  "password": "$CLIENT_PASS"
}
EOF
)
login_response_client=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d "$login_payload_client" \
  "$BASE_URL/auth/login")

# Extract token using jq
CLIENT_TOKEN=$(echo "$login_response_client" | jq -r '.token // empty')

if [[ -z "$CLIENT_TOKEN" ]]; then
  echo "❌ FAILED: Could not extract Client token from login response."
  echo "Response: $login_response_client"
else
  echo "✅ PASSED: Client login successful. Token stored."
fi
echo "--------------------------------------------------"

# 7. Login Trainer Incorrect Password (Public - Expect Unauthorized)
echo "🧪 Test 7: Attempting Trainer login with incorrect password (expect 401)..."
login_payload_trainer_badpass=$(cat <<EOF
{
  "email": "$TRAINER_EMAIL",
  "password": "wrongpassword"
}
EOF
)
expect_status 401 -X POST -H "Content-Type: application/json" -d "$login_payload_trainer_badpass" "$BASE_URL/auth/login"
echo "--------------------------------------------------"

# 8. Access Protected Route (/me) as Trainer (Requires Auth)
echo "🧪 Test 8: Accessing /me endpoint as Trainer..."
if [[ -n "$TRAINER_TOKEN" ]]; then
  # Using expect_status but also printing the response body on success
  me_response_trainer=$(curl -s -w "\n%{http_code}" -X GET \
    -H "Authorization: Bearer $TRAINER_TOKEN" \
    "$BASE_URL/me")
  me_http_code_trainer=$(echo "$me_response_trainer" | tail -n1)
  me_body_trainer=$(echo "$me_response_trainer" | sed '$d') # Get all but last line (status code)

  if [[ "$me_http_code_trainer" -eq 200 ]]; then
    echo "✅ PASSED: Received expected status 200."
    echo "   Response Body: $me_body_trainer"
    # Optional: Validate content with jq
    # echo "$me_body_trainer" | jq -e '.role == "trainer"' > /dev/null && echo "   ✅ Role verified." || echo "   ❌ Role mismatch!"
  else
    echo "❌ FAILED: Expected status 200, but got $me_http_code_trainer for request: curl -H \"Authorization: Bearer <TOKEN>\" $BASE_URL/me"
  fi
else
  echo "⚠️ SKIPPED: Cannot test /me as Trainer because login failed."
fi
echo "--------------------------------------------------"

# 9. Access Protected Route (/me) as Client (Requires Auth)
echo "🧪 Test 9: Accessing /me endpoint as Client..."
if [[ -n "$CLIENT_TOKEN" ]]; then
  me_response_client=$(curl -s -w "\n%{http_code}" -X GET \
    -H "Authorization: Bearer $CLIENT_TOKEN" \
    "$BASE_URL/me")
  me_http_code_client=$(echo "$me_response_client" | tail -n1)
  me_body_client=$(echo "$me_response_client" | sed '$d')

  if [[ "$me_http_code_client" -eq 200 ]]; then
      echo "✅ PASSED: Received expected status 200."
      echo "   Response Body: $me_body_client"
  else
      echo "❌ FAILED: Expected status 200, but got $me_http_code_client for request: curl -H \"Authorization: Bearer <TOKEN>\" $BASE_URL/me"
  fi
else
  echo "⚠️ SKIPPED: Cannot test /me as Client because login failed."
fi
echo "--------------------------------------------------"

# 10. Access Protected Route (/me) without Token (Requires Auth - Expect Unauthorized)
echo "🧪 Test 10: Accessing /me endpoint without token (expect 401)..."
expect_status 401 -X GET "$BASE_URL/me"
echo "--------------------------------------------------"

# 11. Access Protected Route (/me) with Invalid Token (Requires Auth - Expect Unauthorized)
echo "🧪 Test 11: Accessing /me endpoint with invalid token (expect 401)..."
expect_status 401 -X GET -H "Authorization: Bearer invalid.token.string" "$BASE_URL/me"
echo "--------------------------------------------------"

echo "🏁 API Tests Finished."

# TODO: Extend script to test Trainer, Client, and Exercise endpoints
# once their handlers and routes are fully implemented in internal/api/.
# Example: Create exercise (as trainer), get exercises, assign exercise, etc.