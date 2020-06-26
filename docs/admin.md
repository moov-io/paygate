See our [API documentation](https://api.moov.io/admin/paygate/) for Moov PayGate admin endpoints.

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
  "customers": "good",
  "fed": "FED ping failed: Get http://localhost:8086/ping: dial tcp [::1]:8086: connect: connection refused"
}
```

Note: Paygate currently supports `/ready`, but has no checks on this so `200 OK` is always returned.

### Configuration

GET /config

### Reading Micro-Deposits

GET /micro-deposits

### Flushing ACH Files

PUT /trigger-cutoff
