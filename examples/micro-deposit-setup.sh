#!/bin/bash
set -e

teachersFCU='221475786' # ODFI routing number
chaseCO='102001017'

requestID=$(uuidgen)
echo "using requestID ${requestID}"
echo ""

# Create customer
customerID=$(curl -s -H "x-request-id: $requestID" -XPOST "http://localhost:8087/customers" --data '{ "firstName":"Micro", "lastName":"Deposits", "type":"Individual", "email":"valiation@company.io" }' | jq -r .customerID)
echo "Created micro-deposit source customer $customerID"

# Approve customer
customerStatus=$(curl -s -XPUT -H "x-request-id: $requestID" "http://localhost:9097/customers/$customerID/status" --data '{"status": "verified"}' | jq -r .status)
echo "Customer status is $customerStatus"

# Create account for this Customer
accountID=$(curl -s -H "x-request-id: $requestID" -XPOST "http://localhost:8087/customers/$customerID/accounts" --data "{\"accountNumber\": \"123456\", \"routingNumber\": \"$teachersFCU\", \"type\": \"Checking\"}" | jq -r .accountID)
echo "Created customer account $accountID"

# Approve micro-deposit source account
curl -s -H "x-request-id: $requestID" -XPUT "http://localhost:9097/customers/$customerID/accounts/$accountID/status" --data '{"status": "validated"}'
echo "Approved micro-deposit source account"

echo "==========="

echo "In ./examples/config.yaml replace the 'validation:' YAML block with:"
echo "validation:"
echo "  microDeposits:"
echo "    source:"
echo "      customerID: \"$customerID\""
echo "      accountID: \"$accountID\""
echo "==========="
echo ""
echo "Restart PayGate with 'docker-compose up' and run ./examples/micro-deposits.sh"
