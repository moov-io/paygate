moov-io/paygate
===

[![GoDoc](https://godoc.org/github.com/moov-io/paygate?status.svg)](https://godoc.org/github.com/moov-io/paygate)
[![Build Status](https://travis-ci.com/moov-io/paygate.svg?branch=master)](https://travis-ci.com/moov-io/paygate)
[![Coverage Status](https://codecov.io/gh/moov-io/paygate/branch/master/graph/badge.svg)](https://codecov.io/gh/moov-io/paygate)
[![Go Report Card](https://goreportcard.com/badge/github.com/moov-io/paygate)](https://goreportcard.com/report/github.com/moov-io/paygate)
[![Apache 2 licensed](https://img.shields.io/badge/license-Apache2-blue.svg)](https://raw.githubusercontent.com/moov-io/paygate/master/LICENSE)

*project is under active development and is not production ready*

Moov Paygate is a RESTful API enabling Automated Clearing House ([ACH](https://en.wikipedia.org/wiki/Automated_Clearing_House)) transactions to be submitted and received without a deep understanding of a full NACHA file specification.

Docs: [docs.moov.io](https://docs.moov.io/en/latest/) | [api docs](https://editor.swagger.io/?url=https://raw.githubusercontent.com/moov-io/paygate/master/openapi.yaml)

## Project Status

This project is currently pre-production and could change without much notice, however we are looking for community feedback so please try out our code or give us feedback!

## Deployment

You can download [our docker image `moov/paygate`](https://hub.docker.com/r/moov/paygate/) from Docker Hub or use this repository. No configuration is required to serve on `:8082` and metrics at `:9092/metrics` in Prometheus format.

Also, `go run` works:

```
$ cd moov/paygate # wherever this project lives

$ go run .
ts=2018-12-13T19:18:11.970293Z caller=main.go:55 startup="Starting paygate server version v0.1.0-rc3"
ts=2018-12-13T19:18:11.970391Z caller=main.go:59 main="sqlite version 3.25.2"
ts=2018-12-13T19:18:11.971777Z caller=database.go:88 sqlite="starting database migrations"
ts=2018-12-13T19:18:11.971886Z caller=database.go:97 sqlite="migration #0 [create table if not exists receivers(cus...] changed 0 rows"
... (more database migration log lines)
ts=2018-12-13T19:18:11.97221Z caller=database.go:100 sqlite="finished migrations"
ts=2018-12-13T19:18:11.974316Z caller=main.go:96 ach="Pong successful to ACH service"
ts=2018-12-13T19:18:11.975093Z caller=main.go:155 transport=HTTP addr=:8082
ts=2018-12-13T19:18:11.975177Z caller=main.go:124 admin="listening on :9092"
```

### Configuration

- `ACH_ENDPOINT`: DNS record responsible for routing us to an ACH instance. If running as part of our local development setup (or in a Kubernetes cluster we setup) you won't need to set this.
- `GL_ENDPOINT`: A DNS record responsible for routing us to an GL instance. (Example: http://gl.apps.svc.cluster.local:8080)
- `OFAC_ENDPOINT`: HTTP address for HTTP client, defaults to Kubernetes inside clusters and local dev otherwise.
- `OFAC_MATCH_THRESHOLD`: Percent match against OFAC data that's required for paygate to block a transaction.
- `SQLITE_DB_PATH`: Local filepath location for the paygate SQLite database.

#### ACH file uploading / transfers

- `ACH_FILE_BATCH_SIZE`: Number of Transfers to retrieve from the database in each batch for mergin before upload to Fed.
- `ACH_FILE_TRANSFER_INTERVAL`: Go duration for how often to check and sync ACH files on their SFTP destinations.
- `ACH_FILE_STORAGE_DIR`: Filepath for temporary storage of ACH files. This is used as a scratch directory to manage outbound and incoming/returned ACH files.
- `FORCED_CUTOFF_UPLOAD_DELTA`: When the current time is within the routing number's cutoff time by duration force that file to be uploaded.

#### Micro Deposits

In order to validate `Depositories` and transfer money paygate must submit small deposits and credits and have someone confirm the amounts manually. This is only required once per `Depository`. The configuration options for paygate are below and are all required:

- `ODFI_ACCOUNT_NUMBER`: Account Number of Financial Institution which is originating micro deposits.
- `ODFI_BANK_NAME`: Legal name of Financial Institution which is originating micro deposits.
- `ODFI_HOLDER`: Legal name of Financial Institution which is originating micro deposits.
- `ODFI_IDENTIFICATION`: Number by which the customer is known to the Financial Institution originating micro deposits.
- `ODFI_ROUTING_NUMBER`: ABA routing number of Financial Institution which is originating micro deposits.

## Getting Help

 channel | info
 ------- | -------
 [Project Documentation](https://docs.moov.io/en/latest/) | Our project documentation available online.
 Google Group [moov-users](https://groups.google.com/forum/#!forum/moov-users)| The Moov users Google group is for contributors other people contributing to the Moov project. You can join them without a google account by sending an email to [moov-users+subscribe@googlegroups.com](mailto:moov-users+subscribe@googlegroups.com). After receiving the join-request message, you can simply reply to that to confirm the subscription.
Twitter [@moov_io](https://twitter.com/moov_io)	| You can follow Moov.IO's Twitter feed to get updates on our project(s). You can also tweet us questions or just share blogs or stories.
[GitHub Issue](https://github.com/moov-io) | If you are able to reproduce an problem please open a GitHub Issue under the specific project that caused the error.
[moov-io slack](http://moov-io.slack.com/) | Join our slack channel to have an interactive discussion about the development of the project. [Request an invite to the slack channel](https://join.slack.com/t/moov-io/shared_invite/enQtNDE5NzIwNTYxODEwLTRkYTcyZDI5ZTlkZWRjMzlhMWVhMGZlOTZiOTk4MmM3MmRhZDY4OTJiMDVjOTE2MGEyNWYzYzY1MGMyMThiZjg)

## Supported and Tested Platforms

- 64-bit Linux (Ubuntu, Debian), macOS, and Windows

## Contributing

Yes please! Please review our [Contributing guide](CONTRIBUTING.md) and [Code of Conduct](https://github.com/moov-io/ach/blob/master/CODE_OF_CONDUCT.md) to get started!

Note: This project uses Go Modules, which requires Go 1.11 or higher, but we ship the vendor directory in our repository.

### Test Coverage

Improving test coverage is a good candidate for new contributors while also allowing the project to move more quickly by reducing regressions issues that might not be caught before a release is pushed out to our users. One great way to improve coverage is by adding edge cases and different inputs to functions (or [contributing and running fuzzers](https://github.com/dvyukov/go-fuzz)).

Tests can run processes (like sqlite databases), but should only do so locally.

Each PR should increase the overall coverage, if possible. You can run `make cover-test` to save a coverage profile and `make cover-web` to open the HTML view in your default browser.

## License

Apache License 2.0 See [LICENSE](LICENSE) for details.
