#!/bin/bash
set -e

teachersFCU='221475786' # ODFI routing number
chaseCO='102001017'

requestID=$(uuidgen | tr '[:upper:]' '[:lower:]')
echo "using requestID ${requestID}"
echo ""

# Create source customer
sourceCustomerID=$(curl -s -H "x-request-id: $requestID" -XPOST "http://localhost:8087/customers" --data '{ "firstName":"John", "lastName":"Doe", "type":"individual", "email":"john.doe@moov.io" }' | jq -r .customerID)
echo "Created source customer $sourceCustomerID"

# Approve customer
customerStatus=$(curl -s -XPUT -H "x-request-id: $requestID" "http://localhost:9097/customers/$sourceCustomerID/status" --data '{"status": "verified"}' | jq -r .status)
echo "Customer status is $customerStatus"

# Create source account
sourceAccountID=$(curl -s -H "x-request-id: $requestID" -XPOST "http://localhost:8087/customers/$sourceCustomerID/accounts" --data "{\"accountNumber\": \"123456\", \"routingNumber\": \"$teachersFCU\", \"type\": \"Savings\"}" | jq -r .accountID)
echo "Created source customer account $sourceAccountID"

# Approve source account
curl -s -H "x-request-id: $requestID" -XPUT "http://localhost:9097/customers/$sourceCustomerID/accounts/$sourceAccountID/status" --data '{"status": "validated"}'
echo "Approved source account"


echo "==========="


# Create destination customer
destinationCustomerID=$(curl -s -H "x-request-id: $requestID" -XPOST "http://localhost:8087/customers" --data '{ "firstName":"Jane", "lastName":"Doe", "type":"individual", "email":"jane.doe@moov.io" }' | jq -r .customerID)
echo "Created destination customer $destinationCustomerID"

# Approve customer
customerStatus=$(curl -s -XPUT -H "x-request-id: $requestID" "http://localhost:9097/customers/$destinationCustomerID/status" --data '{"status": "verified"}' | jq -r .status)
echo "Customer status is $customerStatus"

# Create destination account
destinationAccountID=$(curl -s -H "x-request-id: $requestID" -XPOST "http://localhost:8087/customers/$destinationCustomerID/accounts" --data "{\"accountNumber\": \"654321\", \"routingNumber\": \"$chaseCO\", \"type\": \"Checking\"}" | jq -r .accountID)
echo "Created destination customer account $destinationAccountID"

# Approve destination account
curl -s -H "x-request-id: $requestID" -XPUT "http://localhost:9097/customers/$destinationCustomerID/accounts/$destinationAccountID/status" --data '{"status": "validated"}'
echo "Approved destination account"


echo "==========="

# Initiate a Transfer
echo "Creating Transfer..."
transferID=$(curl -s -H "x-user-id: moov" -H "x-request-id: $requestID" -XPOST "http://localhost:8082/transfers" --data "{
  \"amount\": \"USD 1.25\",
  \"source\": {
    \"customerID\": \"$sourceCustomerID\",
    \"accountID\": \"$sourceAccountID\"
  },
  \"destination\": {
    \"customerID\": \"$destinationCustomerID\",
    \"accountID\": \"$destinationAccountID\"
  },
  \"description\": \"test transfer\"
}" | jq -r .transferID)

# Trigger our cutoff
sleep 5
curl -XPUT "http://localhost:9092/trigger-cutoff"
sleep 5

# Display transfer
curl -s -H "x-user-id: moov" "http://localhost:8082/transfers/$transferID" | jq -r .

echo "Success! A Transfer was created."
echo ""
echo "An ACH file was uploaded to a test FTP server at ./testdata/ftp-server/outbound/"
ls -l testdata/ftp-server/outbound/
