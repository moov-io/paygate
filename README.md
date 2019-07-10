moov-io/paygate
===

[![GoDoc](https://godoc.org/github.com/moov-io/paygate?status.svg)](https://godoc.org/github.com/moov-io/paygate)
[![Build Status](https://travis-ci.com/moov-io/paygate.svg?branch=master)](https://travis-ci.com/moov-io/paygate)
[![Coverage Status](https://codecov.io/gh/moov-io/paygate/branch/master/graph/badge.svg)](https://codecov.io/gh/moov-io/paygate)
[![Go Report Card](https://goreportcard.com/badge/github.com/moov-io/paygate)](https://goreportcard.com/report/github.com/moov-io/paygate)
[![Apache 2 licensed](https://img.shields.io/badge/license-Apache2-blue.svg)](https://raw.githubusercontent.com/moov-io/paygate/master/LICENSE)

*project is under active development and is not production ready*

Moov Paygate is a RESTful API enabling Automated Clearing House ([ACH](https://en.wikipedia.org/wiki/Automated_Clearing_House)) transactions to be submitted and received without a deep understanding of a full NACHA file specification.

Docs: [docs.moov.io](https://docs.moov.io/paygate/) | [api docs](https://api.moov.io/apps/paygate/)

## Project Status

This project is currently pre-production and could change without much notice, however we are looking for community feedback so please try out our code or give us feedback!

## Getting Started

Paygate can be ran or deployed in various ways. We have several guides for running paygate and offer a testing utility called [`apitest` from the moov-io/api repository](https://github.com/moov-io/api#apitest) for verifying paygate (and its dependnecies) are running properly.

- [Using docker-compose](#local-development)
- [Using our Docker image](#docker-image)
- [Build from source](#build-from-source)

We have some community guides and projects around paygate:

- [How to setup open source ACH payments using Moov.io suite](https://medium.com/@tgunnoe/how-to-setup-open-source-ach-payments-using-moov-io-suite-3586757e45d6) by Taylor Gunnoe
  - Taylor has also written [paygate-cli](https://github.com/tgunnoe/paygate-cli) which is a command-line interface to paygate.

### Local development

We support a [Docker Compose](https://docs.docker.com/compose/gettingstarted/) environment in paygate that can be used to launch the entire Moov stack. After setup launching the stack is the following steps and we have a [tutorial available for running paygate's stack](https://docs.moov.io/paygate/#running-paygate-locally-using-docker-compose-quickest).

### Docker image

You can download [our docker image `moov/paygate`](https://hub.docker.com/r/moov/paygate/) from Docker Hub or use this repository. No configuration is required to serve on `:8082` and metrics at `:9092/metrics` in Prometheus format.

### Build from source

PayGate orchestrates several services that depend on Docker and additional GoLang libraries to run. Paygate leverages [Go Modules](https://github.com/golang/go/wiki/Modules) to manage dependencies. Ensure that your build environment is running Go 1.11 or greater and the environment variable `export GO111MODULE=on` is set. PayGate depends on other Docker containers that will be downloaded for testing and running the service. Ensure [Docker](https://docs.docker.com/get-started/) is installed and running before calling `go run .` in paygate's root directory.

## Deployment

Paygate currently requires the following services to be deployed and available:

- [ACH](https://github.com/moov-io/ach) (HTTP Server) via `ACH_ENDPOINT`
- [FED](https://github.com/moov-io/fed)  (HTTP Server) via `FED_ENDPOINT`
- [OFAC](https://github.com/moov-io/ofac) (HTTP Server) via `OFAC_ENDPOINT`
- The `X-User-Id` (case insensntive) HTTP header is also required and we recommend using an auth proxy to set this. Paygate only expects this value to be unique and consistent to a user.

The following services are required by default, but can be disabled:

- [Accounts](https://github.com/moov-io/accounts) (HTTP server) via `ACCOUNTS_ENDPOINT` and disabled with `ACCOUNTS_CALLS_DISABLED=yes`

### Configuration

The following environmental variables can be set to configure behavior in paygate.

- `ACH_ENDPOINT`: DNS record responsible for routing us to an [ACH](https://github.com/moov-io/ach) instance. If running as part of our local development setup (or in a Kubernetes cluster we setup) you won't need to set this.
- `ACCOUNTS_ENDPOINT`: A DNS record responsible for routing us to an [Accounts](https://github.com/moov-io/accounts) instance. (Example: http://accounts.apps.svc.cluster.local:8080)
  - Set `ACCOUNTS_CALLS_DISABLED=yes` to completely disable all calls to an Accounts service. This is used when paygate doesn't need to integrate with a general ledger solution.
- `FED_ENDPOINT`: HTTP address for [FED](https://github.com/moov-io/fed) interaction to lookup ABA routing numbers
- `HTTP_ADMIN_BIND_ADDRESS`: Address for paygate to bind its admin HTTP server on. This overrides the command-line flag `-admin.addr`. (Default: `:9092`)
- `HTTP_BIND_ADDRESS`: Address for paygate to bind its HTTP server on. This overrides the command-line flag `-http.addr`. (Default: `:8082`)
- `HTTP_CLIENT_CAFILE`: Filepath for additional (CA) certificates to be added into each `http.Client` used within paygate
- `LOG_FORMAT`: Format for logging lines to be written as. (Options: `json`, `plain` - Default: `plain`)
- `OFAC_ENDPOINT`: HTTP address for [OFAC](https://github.com/moov-io/ofac) interaction, defaults to Kubernetes inside clusters and local dev otherwise.
- `OFAC_MATCH_THRESHOLD`: Percent match against OFAC data that's required for paygate to block a transaction.
- `DATABASE_TYPE`: Which database option to use (options: `sqlite` [Default], `mysql`)
  - See **Storage** header below for per-database configuration

#### ACH file uploading / transfers

- `ACH_FILE_BATCH_SIZE`: Number of Transfers to retrieve from the database in each batch for mergin before upload to Fed.
- `ACH_FILE_TRANSFERS_CAFILE`: Filepath for additional (CA) certificates to be added into each FTP client used within paygate
- `ACH_FILE_TRANSFER_INTERVAL`: Go duration for how often to check and sync ACH files on their FTP destinations.
   - Note: Set the value `off` to completely disable async file downloads and uploads.
- `ACH_FILE_STORAGE_DIR`: Filepath for temporary storage of ACH files. This is used as a scratch directory to manage outbound and incoming/returned ACH files.
- `FORCED_CUTOFF_UPLOAD_DELTA`: When the current time is within the routing number's cutoff time by duration force that file to be uploaded.

See [our detailed documentation for FTP configurations](docs/ach.md#ftp-uploads-of-merged-ach-files).

#### Micro Deposits

In order to validate `Depositories` and transfer money paygate must submit small deposits and credits and have someone confirm the amounts manually. This is only required once per `Depository`. The configuration options for paygate are below and are all required:

- `ODFI_ACCOUNT_NUMBER`: Account Number of Financial Institution which is originating micro deposits.
- `ODFI_BANK_NAME`: Legal name of Financial Institution which is originating micro deposits.
- `ODFI_HOLDER`: Legal name of Financial Institution which is originating micro deposits.
- `ODFI_IDENTIFICATION`: Number by which the customer is known to the Financial Institution originating micro deposits.
- `ODFI_ROUTING_NUMBER`: ABA routing number of Financial Institution which is originating micro deposits.

#### Storage

Based on `DATABASE_TYPE` the following environment variables will be read to configure connections for a specific database.

##### MySQL

- `MYSQL_ADDRESS`: TCP address for connecting to the mysql server. (example: `localhost:3306`)
- `MYSQL_DATABASE`: Name of database to connect into.
- `MYSQL_PASSWORD`: Password of user account for authentication.
- `MYSQL_USER`: Username used for authentication,

Refer to the mysql driver documentation for [connection parameters](https://github.com/go-sql-driver/mysql#dsn-data-source-name).

##### SQLite

- `SQLITE_DB_PATH`: Local filepath location for the paygate SQLite database.

Refer to the sqlite driver documentation for [connection parameters](https://github.com/mattn/go-sqlite3#connection-string).

## Getting Help

 channel | info
 ------- | -------
 [Project Documentation](https://docs.moov.io/) | Our project documentation available online.
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
