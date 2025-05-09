#!/bin/bash

# Simple API Test Script for Fitness App

# --- Configuration ---
BASE_URL="http://localhost:8080/api/v1"

# Unique identifiers using timestamp
TIMESTAMP=$(date +%s)
TRAINER_NAME="Test Trainer $TIMESTAMP"
TRAINER_EMAIL="trainer.$TIMESTAMP@example.com"
TRAINER_PASS="password123"

CLIENT_ONE_NAME="Test Client One $TIMESTAMP"
CLIENT_ONE_EMAIL="clientone.$TIMESTAMP@example.com"
CLIENT_ONE_PASS="password456"

CLIENT_TWO_NAME="Test Client Two $TIMESTAMP" # For future tests or manual assignment
CLIENT_TWO_EMAIL="clienttwo.$TIMESTAMP@example.com"
CLIENT_TWO_PASS="password789"


# Variables to store tokens and IDs
TRAINER_TOKEN=""
TRAINER_ID="" # We can get this from the /me endpoint or login response

CLIENT_ONE_TOKEN=""
CLIENT_ONE_ID="" # We can get this from its login response

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
  shift
  local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$@")
  if [[ "$http_code" -ne "$expected_code" ]]; then
    echo "❌ FAILED: Expected status $expected_code, but got $http_code for request: curl $*"
    # Optionally print response body on error:
    # echo "Response body:"
    # curl -s "$@"
    return 1
  else
    echo "✅ PASSED: Received expected status $expected_code."
    return 0
  fi
}

# --- Test Execution ---

check_jq

echo "🚀 Starting API Tests for Fitness App ($BASE_URL)..."
echo "--------------------------------------------------"

# 1. Ping Test
echo "🧪 Test 1: Checking API health (/ping)..."
expect_status 200 "$BASE_URL/ping"
echo "--------------------------------------------------"

# 2. Register Trainer
echo "🧪 Test 2: Registering a new Trainer ($TRAINER_EMAIL)..."
register_payload_trainer=$(cat <<EOF
{ "name": "$TRAINER_NAME", "email": "$TRAINER_EMAIL", "password": "$TRAINER_PASS", "role": "trainer" }
EOF
)
expect_status 201 -X POST -H "Content-Type: application/json" -d "$register_payload_trainer" "$BASE_URL/auth/register"
echo "--------------------------------------------------"

# 3. Register Client One
echo "🧪 Test 3: Registering Client One ($CLIENT_ONE_EMAIL)..."
register_payload_client_one=$(cat <<EOF
{ "name": "$CLIENT_ONE_NAME", "email": "$CLIENT_ONE_EMAIL", "password": "$CLIENT_ONE_PASS", "role": "client" }
EOF
)
expect_status 201 -X POST -H "Content-Type: application/json" -d "$register_payload_client_one" "$BASE_URL/auth/register"
echo "--------------------------------------------------"

# 4. Register Client Two (for potential future use)
echo "🧪 Test 4: Registering Client Two ($CLIENT_TWO_EMAIL)..."
register_payload_client_two=$(cat <<EOF
{ "name": "$CLIENT_TWO_NAME", "email": "$CLIENT_TWO_EMAIL", "password": "$CLIENT_TWO_PASS", "role": "client" }
EOF
)
expect_status 201 -X POST -H "Content-Type: application/json" -d "$register_payload_client_two" "$BASE_URL/auth/register"
echo "--------------------------------------------------"

# 5. Login Trainer
echo "🧪 Test 5: Logging in as Trainer..."
login_payload_trainer=$(cat <<EOF
{ "email": "$TRAINER_EMAIL", "password": "$TRAINER_PASS" }
EOF
)
login_response_trainer=$(curl -s -X POST -H "Content-Type: application/json" -d "$login_payload_trainer" "$BASE_URL/auth/login")
TRAINER_TOKEN=$(echo "$login_response_trainer" | jq -r '.token // empty')
TRAINER_ID=$(echo "$login_response_trainer" | jq -r '.user.id // empty') # Get trainer's ID

if [[ -z "$TRAINER_TOKEN" ]]; then
  echo "❌ FAILED: Could not extract Trainer token."
  exit 1 # Critical failure
else
  echo "✅ PASSED: Trainer login successful. Token stored. Trainer ID: $TRAINER_ID"
fi
echo "--------------------------------------------------"

# 6. Login Client One (to get its ID for verification later, though not strictly needed for add by email)
echo "🧪 Test 6: Logging in as Client One..."
login_payload_client_one=$(cat <<EOF
{ "email": "$CLIENT_ONE_EMAIL", "password": "$CLIENT_ONE_PASS" }
EOF
)
login_response_client_one=$(curl -s -X POST -H "Content-Type: application/json" -d "$login_payload_client_one" "$BASE_URL/auth/login")
CLIENT_ONE_ID=$(echo "$login_response_client_one" | jq -r '.user.id // empty') # Get client's ID

if [[ -z "$CLIENT_ONE_ID" ]]; then
  echo "❌ FAILED: Could not extract Client One ID."
  # Not exiting, as it's not critical for all subsequent tests
else
  echo "✅ PASSED: Client One login successful. Client ID: $CLIENT_ONE_ID"
fi
echo "--------------------------------------------------"


# 7. Trainer: Add Client One by Email (Protected - Trainer Role)
echo "🧪 Test 7: Trainer adds Client One by email..."
if [[ -n "$TRAINER_TOKEN" ]]; then
  add_client_payload=$(cat <<EOF
{ "clientEmail": "$CLIENT_ONE_EMAIL" }
EOF
  )
  add_client_response=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Authorization: Bearer $TRAINER_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$add_client_payload" \
    "$BASE_URL/trainer/clients")
  add_client_http_code=$(echo "$add_client_response" | tail -n1)
  add_client_body=$(echo "$add_client_response" | sed '$d')

  if [[ "$add_client_http_code" -eq 200 ]]; then
    echo "✅ PASSED: Trainer successfully added Client One. Status 200."
    # Verify added client details (optional)
    added_client_email=$(echo "$add_client_body" | jq -r '.email // empty')
    added_client_trainer_id=$(echo "$add_client_body" | jq -r '.trainerId // empty')
    if [[ "$added_client_email" == "$CLIENT_ONE_EMAIL" ]] && [[ "$added_client_trainer_id" == "$TRAINER_ID" ]]; then
        echo "   ✅ Client details in response are correct."
    else
        echo "   ⚠️ WARNING: Client details in add response might be incorrect or missing."
        echo "      Response body: $add_client_body"
    fi
  else
    echo "❌ FAILED: Trainer failed to add Client One. Expected 200, got $add_client_http_code."
    echo "   Response body: $add_client_body"
  fi
else
  echo "⚠️ SKIPPED: Cannot test add client because Trainer login failed."
fi
echo "--------------------------------------------------"

# 8. Trainer: Attempt to Add Non-Existent Client (Protected - Trainer Role)
echo "🧪 Test 8: Trainer attempts to add non-existent client (expect 404)..."
if [[ -n "$TRAINER_TOKEN" ]]; then
  add_non_existent_payload=$(cat <<EOF
{ "clientEmail": "nonexistent.$TIMESTAMP@example.com" }
EOF
  )
  expect_status 404 -X POST \
    -H "Authorization: Bearer $TRAINER_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$add_non_existent_payload" \
    "$BASE_URL/trainer/clients"
else
  echo "⚠️ SKIPPED: Cannot test add non-existent client."
fi
echo "--------------------------------------------------"

# 9. Trainer: Attempt to Add Self as Client (Protected - Trainer Role, expect error, e.g., 403)
echo "🧪 Test 9: Trainer attempts to add self as client (expect 403 - not a client)..."
if [[ -n "$TRAINER_TOKEN" ]]; then
  add_self_payload=$(cat <<EOF
{ "clientEmail": "$TRAINER_EMAIL" }
EOF
  )
  expect_status 403 -X POST \
    -H "Authorization: Bearer $TRAINER_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$add_self_payload" \
    "$BASE_URL/trainer/clients"
else
  echo "⚠️ SKIPPED: Cannot test add self as client."
fi
echo "--------------------------------------------------"


# 10. Trainer: Get Managed Clients (Protected - Trainer Role)
echo "🧪 Test 10: Trainer fetches managed clients..."
if [[ -n "$TRAINER_TOKEN" ]]; then
  get_clients_response=$(curl -s -w "\n%{http_code}" -X GET \
    -H "Authorization: Bearer $TRAINER_TOKEN" \
    "$BASE_URL/trainer/clients")
  get_clients_http_code=$(echo "$get_clients_response" | tail -n1)
  get_clients_body=$(echo "$get_clients_response" | sed '$d')

  if [[ "$get_clients_http_code" -eq 200 ]]; then
    echo "✅ PASSED: Trainer successfully fetched managed clients. Status 200."
    # Verify Client One is in the list
    client_one_found=$(echo "$get_clients_body" | jq --arg email "$CLIENT_ONE_EMAIL" '.[] | select(.email == $email) | .email' | wc -l | tr -d ' ')
    if [[ "$client_one_found" -eq 1 ]]; then
      echo "   ✅ Client One ($CLIENT_ONE_EMAIL) found in the managed list."
    else
      echo "   ❌ FAILED: Client One ($CLIENT_ONE_EMAIL) NOT found in the managed list."
      echo "      Response body: $get_clients_body"
    fi
  else
    echo "❌ FAILED: Trainer failed to fetch managed clients. Expected 200, got $get_clients_http_code."
    echo "   Response body: $get_clients_body"
  fi
else
  echo "⚠️ SKIPPED: Cannot test get managed clients."
fi
echo "--------------------------------------------------"

# 11. Client (non-trainer): Attempt to Add Client (Protected - expect 403 Forbidden)
echo "🧪 Test 11: Client attempts to add another client (expect 403)..."
# First, log in Client One to get a token, if not already done/valid
if [[ -z "$CLIENT_ONE_TOKEN" ]]; then # Simple check, assumes token is still valid if set
    temp_login_resp=$(curl -s -X POST -H "Content-Type: application/json" -d "$login_payload_client_one" "$BASE_URL/auth/login")
    CLIENT_ONE_TOKEN=$(echo "$temp_login_resp" | jq -r '.token // empty')
fi

if [[ -n "$CLIENT_ONE_TOKEN" ]]; then
  add_client_as_client_payload=$(cat <<EOF
{ "clientEmail": "$CLIENT_TWO_EMAIL" }
EOF
  )
  expect_status 403 -X POST \
    -H "Authorization: Bearer $CLIENT_ONE_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$add_client_as_client_payload" \
    "$BASE_URL/trainer/clients" # Using the trainer endpoint
else
  echo "⚠️ SKIPPED: Cannot test client adding client because Client One login failed or token missing."
fi
echo "--------------------------------------------------"


# TODO: Add test case: Trainer tries to add Client One AGAIN (should ideally be idempotent or return a specific message/status)
# Your current service logic might just return 200 OK and the client if already assigned to *this* trainer.

echo "🏁 API Tests Finished."