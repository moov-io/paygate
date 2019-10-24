## Admin Endpoints

### Version

Get the current version of paygate:

```
$ curl localhost:9092/version
v0.6.0
```

### Liveness and Readiness Checks

Liveness checks are included in paygate to aggregate downstream dependency status. If they're available `good` is returned and otherwise an error string is returned. When calling this endpoint checking for the HTTP status of `200 OK` is enough to verify no errors were returned. A 4xx status will be returned if errors are encountered.

```
$ curl -s localhost:9092/live | jq .
{
  "accounts": "good",
  "ach": "good",
  "fed": "FED ping failed: Get http://localhost:8086/ping: dial tcp [::1]:8086: connect: connection refused",
  "ofac": "good"
}
```

Note: Paygate currently supports `/ready`, but has no checks on this so `200 OK` is always returned.


### Flushing ACH File Merging and Uploading

Call this endpoint to start paygate's merging and uploading of ACH files outside of the interval. There's no response except for `200 OK` after the process completes.

```
$ curl -XPOST localhost:9092/files/flush

# paygate logs
ts=2019-08-23T18:36:24.206694Z caller=file_transfer_async.go:218 startPeriodicFileOperations="forcing merge and upload of ACH files"
ts=2019-08-23T18:36:24.206898Z caller=file_transfer_async.go:640 file-transfer-controller="Starting file merge and upload operations"
ts=2019-08-23T18:36:24.207339Z caller=file_transfer_async.go:254 startPeriodicFileOperations="files sync'd, waiting 10m0s"
```

Note: There are endpoints to flush only the incoming or outbound files: `POST /files/flush/incoming` and `POST /files/flush/outgoing`.

### Reading Micro-Deposit Amounts

This endpoint takes a Depository ID and returns the micro-deposits posted against the account.

```
$ curl -H "x-user-id: adam" localhost:9092/depositories/:id/micro-deposits
[{"amount":"USD 0.37"},{"amount":"USD 0.30"}]
```

### ACH File Upload Configs

Paygate has several endpoints for ACH file merging and upload configuration. To view all the configuration call the following endpoint:

```
$ curl -s localhost:9092/configs/uploads | jq .
{
  "CutoffTimes": [
    {
      "RoutingNumber": "121042882",
      "Cutoff": 1700,
      "Location": "America/New_York"
    }
  ],
  "Configs": [
    {
      "RoutingNumber": "121042882",
      "InboundPath": "inbound/",
      "OutboundPath": "outbound/",
      "ReturnPath": "returned/",
      "OutboundFilenameTemplate": ""
    }
  ],
  "FTPConfigs": [
    {
      "RoutingNumber": "121042882",
      "Hostname": "localhost:2121",
      "Username": "admin",
      "Password": "1****6"
    }
  ],
  "SFTPConfigs": [
    {
      "RoutingNumber": "121042882",
      "Hostname": "localhost:2222",
      "Username": "demo",
      "Password": "p******d",
      "ClientPrivateKey": "",
      "HostPublicKey": ""
    }
  ]
}
```

Then you can update (or delete) the `CutoffTime` for a routing number with the following. Config values are unique to a `routingNumber`.

```
$ curl -XPUT localhost:9092/configs/uploads/cutoff-times/{routingNumber} --data '{
    "cutoff": 1700,
    "location": "America/New_York"
}'
```

```
$ curl -XDELETE localhost:9092/configs/uploads/cutoff-times/{routingNumber}
```

Also, update the file transfer configuration (InboundPath, OutboundPath, ReturnPath, etc..). Config values are unique to a `routingNumber`.

```
$ curl -XPUT localhost:9092/configs/uploads/file-transfers/{routingNumber} --data '{"inboundPath": "./inbound/", "outboundPath": "./outbound/", "returnPath": "./returned/"}'
```

```
$ curl -XDELETE localhost:9092/configs/uploads/file-transfers/{routingNumber}
```

#### FTP (File Transfer Protocol)

Update the hostname, username, password, etc for a routing number's FTP config. The `password` is optional, but all other fields are required. Config values are unique to a `routingNumber`.

```
$ curl -XPUT localhost:9092/configs/uploads/ftp/{routingNumber} --data '{
    "hostname": "...",
    "username": "...",
    "password": "..." // optional
}'
```

```
$ curl -XDELETE localhost:9092/configs/uploads/ftp/{routingNumber}
```

#### SFTP (SSH File Transfer Protocol)

Update the hostname, username, password, client private key, host public key, etc for a routing number's SFTP config. `password`, `clientPrivateKey`, and `hostPublicKey` are all optional, but all other fields are required. Config values are unique to a `routingNumber`.

```
$ curl -XPUT localhost:9092/configs/uploads/sftp/{routingNumber} --data '{
    "hostname": "...",
    "username": "...",
    "password": "...",         // optional
    "clientPrivateKey": "...", // optional
    "hostPublicKey": "...",    // optional
}'
```

```
$ curl -XDELETE localhost:9092/configs/uploads/sftp/{routingNumber}
```
