#!/bin/bash

# -----------------------------------------------------------------------------
# Load environment variables from .env file if it exists
if [ -f .env ]; then
  export $(echo $(cat .env | sed 's/#.*//g'| xargs) | envsubst)
fi

# Check if the variable is set
if [ -z "$POSTGRES_URL" ]; then
    echo "Error: POSTGRES_URL is not set. Please create a .env file."
    exit 1
fi

# Check if NATS CLI is available
if ! command -v nats &> /dev/null; then
    echo "Error: NATS CLI is not installed. Please install it first."
    echo "Install with: go install github.com/nats-io/natscli/nats@latest"
    exit 1
fi
# -----------------------------------------------------------------------------

# --- Create a fabric to be updated via REST API
echo "--- CREATING a new fabric (TEST01) via REST API ---"
curl -i -X POST "localhost:8080/v1/fabrics" \
-H "Content-Type: application/json" \
-d '{
    "code": "TEST01",
    "name": "Fabric From POST request",
    "measure_unit": "mb",
    "offer_status": "available"
}'
echo -e "\n"

# --- Test fabric creation via NATS event
echo "--- CREATING a new fabric (TEST02) via NATS event ---"
nats pub "fabrics.events" '{
  "event_id": "test-create-002",
  "event_type": "fabric.created",
  "aggregate_id": "TEST02",
  "aggregate_type": "fabric",
  "aggregate_version": 1,
  "event_version": 1,
  "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
  "correlation_id": "test-corr-002",
  "payload": {
    "operation": "create",
    "kod": "TEST02",
    "nazwa": "Fabric From NATS Event",
    "version": 1,
    "measure_unit": "PCS",
    "offer_status": "ACTIVE"
  }
}'
echo "NATS event sent for fabric creation"
sleep 2  # Give time for async processing
echo -e "\n"

# --- Test the UPDATE endpoint via REST
echo "--- UPDATING fabric (TEST01) via REST API ---"
curl -i -X PUT "localhost:8080/v1/fabrics/TEST01" \
-H "Content-Type: application/json" \
-d '{
    "name": "UPDATED Fabric Name",
    "measure_unit": "cm",
    "offer_status": "unavailable",
    "version": 1
}'
echo -e "\n"

# --- Test the UPDATE endpoint to get 409 conflict code (version)
echo "--- CONFLICT ON UPDATING fabric (TEST01) via REST API ---"
curl -i -X PUT "localhost:8080/v1/fabrics/TEST01" \
-H "Content-Type: application/json" \
-d '{
    "name": "FAILED UPDATE Fabric Name",
    "measure_unit": "cm",
    "offer_status": "unavailable",
    "version": 1
}'
echo -e "\n"

# --- Test for Delete and Reactivate API Lifecycle ---
echo "--- LIFECYCLE: DELETE AND REACTIVATE via REST API ---"
echo "--- CREATING fabric to be deleted (TEST03) ---"
curl -i -X POST "localhost:8080/v1/fabrics" \
-H "Content-Type: application/json" \
-d '{
    "code": "TEST03",
    "name": "Lifecycle Test",
    "measure_unit": "m",
    "offer_status": "available"
}'
echo -e "\n"

echo "--- DELETING fabric (TEST03) with version 1 ---"
curl -i -X DELETE "localhost:8080/v1/fabrics/TEST03" \
-H "Content-Type: application/json" \
-d '{
    "version": 1
}'
echo -e "\n"

echo "--- VERIFYING fabric (TEST03) is deleted (expect 404) ---"
curl -i "localhost:8080/v1/fabrics/TEST03"
echo -e "\n"

echo "--- REACTIVATING fabric (TEST03) by creating it again ---"
curl -i -X POST "localhost:8080/v1/fabrics" \
-H "Content-Type: application/json" \
-d '{
    "code": "TEST03",
    "name": "Reactivated Lifecycle Test",
    "measure_unit": "m",
    "offer_status": "available"
}'
echo -e "\n"

# --- Test for Delete and Reactivate via NATS Events ---
echo "--- LIFECYCLE: DELETE AND REACTIVATE via NATS Events ---"
echo "--- CREATING fabric to be deleted (TEST04) ---"
curl -i -X POST "localhost:8080/v1/fabrics" \
-H "Content-Type: application/json" \
-d '{
    "code": "TEST04",
    "name": "NATS Lifecycle Test",
    "measure_unit": "m",
    "offer_status": "available"
}'
echo -e "\n"

echo "--- DELETING fabric (TEST04) with version 1 via NATS event ---"
nats pub "fabrics.events" '{
  "event_id": "test-delete-004",
  "event_type": "fabric.deleted",
  "aggregate_id": "TEST04",
  "aggregate_type": "fabric",
  "aggregate_version": 2,
  "event_version": 1,
  "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
  "correlation_id": "test-corr-delete-004",
  "payload": {
    "operation": "delete",
    "kod": "TEST04",
    "nazwa": "NATS Lifecycle Test",
    "version": 1
  }
}'
echo "NATS delete event sent"
sleep 2  # Give time for async processing
echo -e "\n"

echo "--- VERIFYING fabric (TEST04) is deleted (expect 404) ---"
curl -i "localhost:8080/v1/fabrics/TEST04"
echo -e "\n"

echo "--- REACTIVATING fabric (TEST04) by sending a NATS create event ---"
nats pub "fabrics.events" '{
  "event_id": "test-recreate-004",
  "event_type": "fabric.created",
  "aggregate_id": "TEST04",
  "aggregate_type": "fabric",
  "aggregate_version": 1,
  "event_version": 1,
  "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
  "correlation_id": "test-corr-recreate-004",
  "payload": {
    "operation": "create",
    "kod": "TEST04",
    "nazwa": "Reactivated NATS Lifecycle Test",
    "version": 1,
    "measure_unit": "METER",
    "offer_status": "ACTIVE"
  }
}'
echo "NATS recreation event sent"
sleep 2  # Give time for async processing
echo -e "\n"

echo "--- LIFECYCLE: NATS UPDATE ---"
echo "--- CREATING fabric to be updated (TEST05) ---"
curl -i -X POST "localhost:8080/v1/fabrics" \
-H "Content-Type: application/json" \
-d '{
    "code": "TEST05",
    "name": "NATS Update Test",
    "measure_unit": "m",
    "offer_status": "available"
}'
echo -e "\n"

echo "--- UPDATING fabric (TEST05) via NATS event ---"
nats pub "fabrics.events" '{
  "event_id": "test-update-005",
  "event_type": "fabric.updated",
  "aggregate_id": "TEST05",
  "aggregate_type": "fabric",
  "aggregate_version": 2,
  "event_version": 1,
  "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
  "correlation_id": "test-corr-update-005",
  "payload": {
    "operation": "update",
    "kod": "TEST05",
    "nazwa": "Updated by NATS Event",
    "version": 1,
    "measure_unit": "CENTIMETER",
    "offer_status": "INACTIVE"
  }
}'
echo "NATS update event sent"
sleep 2  # Give time for async processing
echo -e "\n"

# --- Test edge cases with NATS events ---
echo "--- TESTING edge cases with NATS events ---"

echo "--- Testing fabric with minimal payload (missing measure_unit and offer_status) ---"
nats pub "fabrics.events" '{
  "event_id": "test-minimal-006",
  "event_type": "fabric.created",
  "aggregate_id": "TEST06",
  "aggregate_type": "fabric",
  "aggregate_version": 1,
  "event_version": 1,
  "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
  "correlation_id": "test-corr-minimal-006",
  "payload": {
    "operation": "create",
    "kod": "TEST06",
    "nazwa": "Minimal Payload Test",
    "version": 1
  }
}'
echo "NATS minimal payload event sent"
sleep 2
echo -e "\n"

echo "--- Testing fabric with invalid data (should fail validation) ---"
nats pub "fabrics.events" '{
  "event_id": "test-invalid-007",
  "event_type": "fabric.created",
  "aggregate_id": "test-invalid",
  "aggregate_type": "fabric",
  "aggregate_version": 1,
  "event_version": 1,
  "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
  "correlation_id": "test-corr-invalid-007",
  "payload": {
    "operation": "create",
    "kod": "test-invalid-lowercase",
    "nazwa": "",
    "version": 0
  }
}'
echo "NATS invalid data event sent (should be rejected)"
sleep 2
echo -e "\n"

# Verify the final state of the fabrics
echo "--- VERIFYING final state ---"
echo "Fetching TEST01 (should be updated via REST):"
curl -i "localhost:8080/v1/fabrics/TEST01"
echo -e "\n"
echo "Fetching TEST02 (should be created by NATS):"
curl -i "localhost:8080/v1/fabrics/TEST02"
echo -e "\n"
echo "Fetching TEST03 (should be active again via REST):"
curl -i "localhost:8080/v1/fabrics/TEST03"
echo -e "\n"
echo "Fetching TEST04 (should be active again via NATS):"
curl -i "localhost:8080/v1/fabrics/TEST04"
echo -e "\n"
echo "Fetching TEST05 (should be updated by NATS):"
curl -i "localhost:8080/v1/fabrics/TEST05"
echo -e "\n"
echo "Fetching TEST06 (should be created with defaults by NATS):"
curl -i "localhost:8080/v1/fabrics/TEST06"
echo -e "\n"
echo "Fetching TEST-INVALID (should not exist due to validation failure):"
curl -i "localhost:8080/v1/fabrics/test-invalid-lowercase"
echo -e "\n"

# Check the event store
echo "--- CHECKING events in database ---"
psql $POSTGRES_URL -c "SELECT aggregate_id, event_type, aggregate_version, timestamp FROM events WHERE aggregate_id LIKE 'TEST%' ORDER BY timestamp;"
echo -e "\n"

# Cleanup
echo "--- CLEANING up test data ---"
psql $POSTGRES_URL -c "DELETE FROM fabrics WHERE code LIKE 'TEST%';"
psql $POSTGRES_URL -c "DELETE FROM events WHERE aggregate_id LIKE 'TEST%';"

echo "--- TEST COMPLETED ---"
echo "Check your application logs to see how NATS events were processed!"