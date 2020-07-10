#!/bin/bash
set -e

teachersFCU='221475786' # ODFI routing number
chaseCO='102001017'

requestID=$(uuidgen)
echo "using requestID ${requestID}"
echo ""

# Create destination customer
destinationCustomerID=$(curl -s -H "x-request-id: $requestID" -XPOST "http://localhost:8087/customers" --data '{ "firstName":"Jane", "lastName":"Doe", "type":"individual", "email":"jane.doe@moov.io" }' | jq -r .customerID)
echo "Created customer $destinationCustomerID to approve"

# Approve customer
customerStatus=$(curl -s -XPUT -H "x-request-id: $requestID" "http://localhost:9097/customers/$destinationCustomerID/status" --data '{"status": "verified"}' | jq -r .status)
echo "Customer status is $customerStatus"

# Create destination account
destinationAccountID=$(curl -s -H "x-request-id: $requestID" -XPOST "http://localhost:8087/customers/$destinationCustomerID/accounts" --data "{\"accountNumber\": \"654321\", \"routingNumber\": \"$chaseCO\", \"type\": \"Checking\"}" | jq -r .accountID)
echo "Created destination customer account $destinationAccountID"

# Validate account with micro-deposits
echo "Initiating micro-deposits..."
curl -H "x-request-id: $requestID" -H "x-user-id: moov" -XPUT "http://localhost:8087/customers/$destinationCustomerID/accounts/$destinationAccountID/validate" --data '{"strategy": "micro-deposits"}'
echo "Initiated micro-deposits for destination account"
sleep 5

# Trigger our cutoff
curl -XPUT "http://localhost:9092/trigger-cutoff"
sleep 5

# Read micro-deposits
microDepositAmounts=$(curl -s -H "x-request-id: $requestID" -H "x-user-id: moov" "http://localhost:8082/accounts/$destinationAccountID/micro-deposits" | jq -r .amounts)
echo "Found micro-deposits: $microDepositAmounts"
curl -s -H "x-request-id: $requestID" -H "x-user-id: moov" -XPUT "http://localhost:8087/customers/$destinationCustomerID/accounts/$destinationAccountID/validate" --data "{\"strategy\": \"micro-deposits\", \"micro-deposits\": $microDepositAmounts}"

# Grab the account status
destinationAccounts=$(curl -s -H "x-request-id: $requestID" "http://localhost:8087/customers/$destinationCustomerID/accounts" | jq -r .)
echo "Customer accounts: $destinationAccounts"

echo "Success! The account was validated with micro-deposits"
echo ""
echo "An ACH file was uploaded to a test FTP server at ./testdata/ftp-server/outbound/"
ls -l testdata/ftp-server/outbound/
