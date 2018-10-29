#!/bin/bash
set -e

# Notes
# 121042882 // Wells Fargo
# 231380104 // Citadel

requestId=$(uuidgen)
userId=$(whoami)

echo "Using X-Request-Id: $requestId"

mkdir -p /tmp/paygate/

# Create Originator Depository
curl -s -o /tmp/paygate/origDep.json -XPOST -H "x-user-id: $userId" -H "x-request-id: $requestId" http://localhost:8082/depositories --data '{"bankName":"orig bank", "holder": "me", "holderType": "individual", "type": "Checking", "routingNumber": "121042882", "accountNumber": "123"}'

origDepId=$(jq -r '.id' /tmp/paygate/origDep.json)
echo "Created Originator Depository: $origDepId"

# Create Originator
curl -s -o /tmp/paygate/orig.json -XPOST -H "x-user-id: $userId" -H "x-request-id: $requestId" http://localhost:8082/originators --data "{\"defaultDepository\": \"$origDepId\", \"identification\": \"secret\"}"

orig=$(jq -r '.id' /tmp/paygate/orig.json)
echo "Created Originator: $orig"

# Create Customer Depository
curl -s -o /tmp/paygate/custDep.json -XPOST -H "x-user-id: $userId" -H "x-request-id: $requestId" http://localhost:8082/depositories --data '{"bankName":"cust bank", "holder": "you", "holderType": "individual", "type": "Checking", "routingNumber": "231380104", "accountNumber": "451"}'

custDepId=$(jq -r '.id' /tmp/paygate/custDep.json)
echo "Created Customer Depository: $custDepId"

# Create Customer
curl -s -o /tmp/paygate/cust.json -XPOST -H "x-user-id: $userId" -H "x-request-id: $requestId" http://localhost:8082/customers --data "{\"defaultDepository\": \"$custDepId\", \"email\": \"test@moov.io\"}"

cust=$(jq -r '.id' /tmp/paygate/cust.json)
echo "Created Customer: $cust"

# Create Transfer
curl -s -o /tmp/paygate/transfer.json -XPOST -H "x-user-id: $userId" -H "x-request-id: $requestId" http://localhost:8082/transfers --data "{\"transferType\": \"push\", \"amount\": \"USD 78.54\", \"originator\": \"$orig\", \"originatorDepository\": \"$origDepId\", \"customer\": \"$cust\", \"customerDepository\": \"$custDepId\", \"description\": \"test payment\", \"standardEntryClassCode\": \"PPD\"}"

transferId=$(jq -r '.[] | .id' /tmp/paygate/transfer.json)
if [ "$transferId" == "null" ]; then
    jq . /tmp/paygate/transfer.json
else
    echo "Created Transfer: $transferId"
fi
