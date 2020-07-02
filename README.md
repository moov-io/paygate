moov-io/paygate
===

[![GoDoc](https://godoc.org/github.com/moov-io/paygate?status.svg)](https://godoc.org/github.com/moov-io/paygate)
[![Build Status](https://github.com/moov-io/paygate/workflows/Go/badge.svg)](https://github.com/moov-io/paygate/actions)
[![Coverage Status](https://codecov.io/gh/moov-io/paygate/branch/master/graph/badge.svg)](https://codecov.io/gh/moov-io/paygate)
[![Go Report Card](https://goreportcard.com/badge/github.com/moov-io/paygate)](https://goreportcard.com/report/github.com/moov-io/paygate)
[![Apache 2 licensed](https://img.shields.io/badge/license-Apache2-blue.svg)](https://raw.githubusercontent.com/moov-io/paygate/master/LICENSE)

Moov Paygate is a RESTful API enabling Automated Clearing House ([ACH](https://en.wikipedia.org/wiki/Automated_Clearing_House)) transactions to be submitted and received without a deep understanding of a full NACHA file specification.

Docs: [Project](https://github.com/moov-io/paygate/tree/master/docs/) | [API Endpoints](https://moov-io.github.io/paygate/) | [Admin API Endpoints](https://moov-io.github.io/paygate/admin/)

This project is sponsored by Moov Financial, Inc which offers commercial support, hosting, and additional features. Refer to our [product documentation](https://docs.moov.io/paygate/) for more information.

## Project Status

This project is currently under development and could introduce breaking changes to reach a stable status. We are looking for community feedback so please try out our code or give us feedback!

## Getting Started

Paygate can be ran or deployed in various ways. We have several guides for running paygate and offer a testing utility called [`apitest` from the moov-io/api repository](https://github.com/moov-io/api#apitest) for verifying paygate (and its dependnecies) are running properly.

- [Using docker-compose](#local-development)
- [Using our Docker image](#docker-image)
- [Build from source](#build-from-source)
- [How to setup open source ACH payments using Moov.io suite](https://medium.com/@tgunnoe/how-to-setup-open-source-ach-payments-using-moov-io-suite-3586757e45d6) by Taylor Gunnoe
  - Taylor has also written [paygate-cli](https://github.com/tgunnoe/paygate-cli) which is a command-line interface to paygate.

## Deployment

Paygate currently requires an instance of [Moov Customers](https://github.com/moov-io/customers) to be running and the address to its HTTP server set in PayGate's config file.

Note: The `X-User-Id` (case insensntive) HTTP header is also required and we recommend using an auth proxy to set this. Paygate only expects this value to be unique and consistent to a user.

### Docker image

You can download [our docker image `moov/paygate`](https://hub.docker.com/r/moov/paygate/) from Docker Hub or use this repository. No configuration is required to serve on `:8082` and metrics at `:9092/metrics` in Prometheus format.


```
$ docker run -p 8082:8082 moov/paygate:latest
ts=2020-06-29T21:57:08.427368Z caller=main.go:243 startup="Starting paygate server version v0.8.0"
level=info ts=2020-06-29T21:57:08.428856Z caller=logger.go:63 msg="Initializing logging reporter\n"
ts=2020-06-29T21:57:08.429215Z caller=database.go:25 database="setting up sqlite database provider"
ts=2020-06-29T21:57:08.429252Z caller=sqlite.go:97 main="sqlite version 3.31.1"
ts=2020-06-29T21:57:08.43175Z caller=main.go:87 admin="listening on [::]:9092"
ts=2020-06-29T21:57:08.433259Z caller=main.go:134 main="registered America/New_York cutoffs=16:20"
ts=2020-06-29T21:57:08.43329Z caller=aggregate.go:72 aggregate="setup *audittrail.MockStorage audit storage"
ts=2020-06-29T21:57:08.433309Z caller=aggregate.go:78 aggregate="setup []transform.PreUpload(nil) pre-upload transformers"
ts=2020-06-29T21:57:08.433317Z caller=aggregate.go:84 aggregate="setup *output.NACHA output formatter"
ts=2020-06-29T21:57:08.433379Z caller=client.go:186 customers="using http://localhost:8087 for Customers address"
ts=2020-06-29T21:57:08.433623Z caller=scheduler.go:44 inbound="starting inbound processor with interval=10m0s"
ts=2020-06-29T21:57:08.433635Z caller=main.go:207 startup="binding to :8082 for HTTP server"
```

### Local development

We support a [Docker Compose](https://docs.docker.com/compose/gettingstarted/) environment in paygate that can be used to launch the entire Moov stack. Using the source code of the [latest released `docker-compose.yml`](https://github.com/moov-io/paygate/releases/latest) is recommended.

```
$ docker-compose up -d
paygate_ach_1 is up-to-date
paygate_ofac_1 is up-to-date
Recreating paygate_accounts_1 ...
paygate_fed_1 is up-to-date
Recreating paygate_accounts_1 ... done
Recreating paygate_paygate_1  ... done
```

### Build from source

PayGate orchestrates several services that depend on Docker and additional GoLang libraries to run. Paygate leverages [Go Modules](https://github.com/golang/go/wiki/Modules) to manage dependencies. Ensure that your build environment is running Go 1.14 or greater. PayGate depends on other Docker containers that will be downloaded for testing and running the service. Ensure [Docker](https://docs.docker.com/get-started/) is installed and running.

```
$ cd moov/paygate # wherever this project lives

$ go run ./cmd/server/
ts=2020-06-29T21:57:08.427368Z caller=main.go:243 startup="Starting paygate server version v0.8.0-dev"
level=info ts=2020-06-29T21:57:08.428856Z caller=logger.go:63 msg="Initializing logging reporter\n"
ts=2020-06-29T21:57:08.429215Z caller=database.go:25 database="setting up sqlite database provider"
ts=2020-06-29T21:57:08.429252Z caller=sqlite.go:97 main="sqlite version 3.31.1"
ts=2020-06-29T21:57:08.43175Z caller=main.go:87 admin="listening on [::]:9092"
ts=2020-06-29T21:57:08.433259Z caller=main.go:134 main="registered America/New_York cutoffs=16:20"
ts=2020-06-29T21:57:08.43329Z caller=aggregate.go:72 aggregate="setup *audittrail.MockStorage audit storage"
ts=2020-06-29T21:57:08.433309Z caller=aggregate.go:78 aggregate="setup []transform.PreUpload(nil) pre-upload transformers"
ts=2020-06-29T21:57:08.433317Z caller=aggregate.go:84 aggregate="setup *output.NACHA output formatter"
ts=2020-06-29T21:57:08.433379Z caller=client.go:186 customers="using http://localhost:8087 for Customers address"
ts=2020-06-29T21:57:08.433623Z caller=scheduler.go:44 inbound="starting inbound processor with interval=10m0s"
ts=2020-06-29T21:57:08.433635Z caller=main.go:207 startup="binding to :8082 for HTTP server"
```

## Getting Help

 channel | info
 ------- | -------
 [Project Documentation](https://github.com/moov-io/paygate/tree/master/docs/) | Our project documentation available online.
 [Hosted Documentation](https://docs.moov.io/paygate/) | Hosted documentation for enterprise solutions.
 Google Group [moov-users](https://groups.google.com/forum/#!forum/moov-users)| The Moov users Google group is for contributors other people contributing to the Moov project. You can join them without a google account by sending an email to [moov-users+subscribe@googlegroups.com](mailto:moov-users+subscribe@googlegroups.com). After receiving the join-request message, you can simply reply to that to confirm the subscription.
Twitter [@moov_io](https://twitter.com/moov_io)	| You can follow Moov.IO's Twitter feed to get updates on our project(s). You can also tweet us questions or just share blogs or stories.
[GitHub Issue](https://github.com/moov-io) | If you are able to reproduce a problem please open a GitHub Issue under the specific project that caused the error.
[moov-io slack](https://slack.moov.io/) | Join our slack channel (`#paygate`) to have an interactive discussion about the development of the project.

## Supported and Tested Platforms

- 64-bit Linux (Ubuntu, Debian), macOS, and Windows

## Contributing

Yes please! Please review our [Contributing guide](CONTRIBUTING.md) and [Code of Conduct](https://github.com/moov-io/ach/blob/master/CODE_OF_CONDUCT.md) to get started! Checkout our [issues for first time contributors](https://github.com/moov-io/paygate/contribute) for something to help out with.

Paygate includes several "long" tests which spawn Docker containers, make requests over the internet, and perform more complicated tests. To skip these long tests add the `-short` flag to `go test`.

This project uses [Go Modules](https://github.com/golang/go/wiki/Modules) and uses Go 1.14 or higher. See [Golang's install instructions](https://golang.org/doc/install) for help setting up Go. You can download the source code and we offer [tagged and released versions](https://github.com/moov-io/paygate/releases/latest) as well. We highly recommend you use a tagged release for production.

### Further Reading

- [The default banking-as-a-service platform will be developer-first](https://www.kunle.app/feb-2020-permissionless-issuing.html)

### Test Coverage

Improving test coverage is a good candidate for new contributors while also allowing the project to move more quickly by reducing regressions issues that might not be caught before a release is pushed out to our users. One great way to improve coverage is by adding edge cases and different inputs to functions (or [contributing and running fuzzers](https://github.com/dvyukov/go-fuzz)).

Tests can run processes (like sqlite databases), but should only do so locally.

Each PR should increase the overall coverage, if possible. You can run `make cover-test` to save a coverage profile and `make cover-web` to open the HTML view in your default browser.

## License

Apache License 2.0 See [LICENSE](LICENSE) for details.
