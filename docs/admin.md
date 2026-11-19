See our [admin API documentation](https://moov-io.github.io/paygate/admin/) for Moov PayGate admin endpoints.

### Version

Get the current version of paygate:

```
$ curl localhost:9092/version
v0.10.4
```

### Liveness and Readiness Checks

Liveness checks are included in paygate to aggregate downstream dependency status. If they're available `good` is returned and otherwise an error string is returned. When calling this endpoint checking for the HTTP status of `200 OK` is enough to verify no errors were returned. A 4xx status will be returned if errors are encountered.

```
$ curl -s localhost:9092/live | jq .
{
  "customers": "good",
  "ftp": "good"
}
```

The exact keys depend on your config. PayGate registers a liveness check for the configured upload agent (`ftp` or `sftp`) and for Customers. A micro-deposits check is added when that validation source is configured.

Note: Paygate currently supports `/ready`, but has no checks on this so `200 OK` is always returned.

### Configuration

PayGate offers an endpoint for retrieving the config object from a running instance. Credentials and other secrets are returned in a masked form. Set `admin.disableConfigEndpoint: true` to turn the endpoint off.

Refer to the [documentation for this config](./config.md) for full details.

```
$ curl -s http://localhost:9092/config | jq .
{
  "logging": {
    "format": "plain",
    "level": ""
  },
  "http": {
    // ...
  },
  "admin": {
    // ...
  },
  "database": {
    // ...
  },
  "odfi": {
    // ..
  },
  "pipeline": {
    // ...
  },
  "validation": {
    // ...
  },
  "customers": {
    // ...
  }
}

```

### Flushing ACH Files

There is an endpoint to initiate cutoff processing as if a window has approached. This involves merging transfers into files, upload attempts, along with inbound file download processing.

```
$ curl -XPUT http://localhost:9092/trigger-cutoff
// check for errors, or '200 OK'
```
