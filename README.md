moov-io/paygate
===

[![GoDoc](https://godoc.org/github.com/moov-io/paygate?status.svg)](https://godoc.org/github.com/moov-io/paygate)
[![Build Status](https://travis-ci.com/moov-io/paygate.svg?branch=master)](https://travis-ci.com/moov-io/paygate)
[![Coverage Status](https://codecov.io/gh/moov-io/paygate/branch/master/graph/badge.svg)](https://codecov.io/gh/moov-io/paygate)
[![Go Report Card](https://goreportcard.com/badge/github.com/moov-io/paygate)](https://goreportcard.com/report/github.com/moov-io/paygate)
[![Apache 2 licensed](https://img.shields.io/badge/license-Apache2-blue.svg)](https://raw.githubusercontent.com/moov-io/paygate/master/LICENSE)

Moov Paygate is a RESTful API enabling Automated Clearing House ([ACH](https://en.wikipedia.org/wiki/Automated_Clearing_House)) transactions to be submitted and received without a deep understanding of a full NACHA file specification.

Docs: [docs.moov.io](https://docs.moov.io/paygate/) | [api docs](https://moov-io.github.io/paygate/) | [admin api docs](https://moov-io.github.io/paygate/admin/)

## Project Status

This project is currently under development and could introduce breaking changes to reach a stable status. We are looking for community feedback so please try out our code or give us feedback!

The latest stable release is on the v0.7.x series. Please [use that branch](https://github.com/moov-io/paygate/tree/release-v0.7) if you're building from source.

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
ts=2018-12-13T19:18:11.970293Z caller=main.go:55 startup="Starting paygate server version v0.5.1"
ts=2018-12-13T19:18:11.970391Z caller=main.go:59 main="sqlite version 3.25.2"
ts=2018-12-13T19:18:11.971777Z caller=database.go:88 sqlite="starting database migrations"
ts=2018-12-13T19:18:11.971886Z caller=database.go:97 sqlite="migration #0 [create table if not exists receivers(cus...] changed 0 rows"
... (more database migration log lines)
ts=2018-12-13T19:18:11.97221Z caller=database.go:100 sqlite="finished migrations"
ts=2018-12-13T19:18:11.974316Z caller=main.go:96 ach="Pong successful to ACH service"
ts=2018-12-13T19:18:11.975093Z caller=main.go:155 transport=HTTP addr=:8082
ts=2018-12-13T19:18:11.975177Z caller=main.go:124 admin="listening on :9092"

$ curl -XPOST -H "x-user-id: test" localhost:8082/originators --data '{...}'
```

### Local development

We support a [Docker Compose](https://docs.docker.com/compose/gettingstarted/) environment in paygate that can be used to launch the entire Moov stack. After setup launching the stack is the following steps and we offer a testing utility [`apitest` from the moov-io/api repository](https://github.com/moov-io/api#apitest).

Using the [latest released `docker-compose.yml`](https://github.com/moov-io/paygate/releases/latest) is recommended as that will use released versions of dependencies.

```
$ docker-compose up -d
paygate_ach_1 is up-to-date
paygate_ofac_1 is up-to-date
Recreating paygate_accounts_1 ...
paygate_fed_1 is up-to-date
Recreating paygate_accounts_1 ... done
Recreating paygate_paygate_1  ... done

# Run Moov's testing utility
$ apitest -local
2019/06/10 21:18:06.117261 main.go:61: Starting apitest v0.9.5
2019/06/10 21:18:06.117293 main.go:133: Using http://localhost as base API address
...
2019/06/10 21:18:06.276443 main.go:218: SUCCESS: Created user b1f2671bbed52ed6da88f16ce467cadecb0ee1b6 (email: festive.curran27@example.com)
...
2019/06/10 21:18:06.607817 main.go:218: SUCCESS: Created USD 204.71 transfer (id=b7ecb109574187ff726ba48275dcf88956c26841) for user
```

### Build from source

PayGate orchestrates several services that depend on Docker and additional GoLang libraries to run. Paygate leverages [Go Modules](https://github.com/golang/go/wiki/Modules) to manage dependencies. Ensure that your build environment is running Go 1.14 or greater. PayGate depends on other Docker containers that will be downloaded for testing and running the service. Ensure [Docker](https://docs.docker.com/get-started/) is installed and running.

```
$ cd moov/paygate # wherever this project lives

$ go run .
ts=2018-12-13T19:18:11.970293Z caller=main.go:55 startup="Starting paygate server version v0.5.1"
ts=2018-12-13T19:18:11.970391Z caller=main.go:59 main="sqlite version 3.25.2"
ts=2018-12-13T19:18:11.971777Z caller=database.go:88 sqlite="starting database migrations"
ts=2018-12-13T19:18:11.971886Z caller=database.go:97 sqlite="migration #0 [create table if not exists receivers(cus...] changed 0 rows"
... (more database migration log lines)
ts=2018-12-13T19:18:11.97221Z caller=database.go:100 sqlite="finished migrations"
ts=2018-12-13T19:18:11.974316Z caller=main.go:96 ach="Pong successful to ACH service"
ts=2018-12-13T19:18:11.975093Z caller=main.go:155 transport=HTTP addr=:8082
ts=2018-12-13T19:18:11.975177Z caller=main.go:124 admin="listening on :9092"
```

## Getting Help

 channel | info
 ------- | -------
 [Project Documentation](https://docs.moov.io/) | Our project documentation available online.
 Google Group [moov-users](https://groups.google.com/forum/#!forum/moov-users)| The Moov users Google group is for contributors other people contributing to the Moov project. You can join them without a google account by sending an email to [moov-users+subscribe@googlegroups.com](mailto:moov-users+subscribe@googlegroups.com). After receiving the join-request message, you can simply reply to that to confirm the subscription.
Twitter [@moov_io](https://twitter.com/moov_io)	| You can follow Moov.IO's Twitter feed to get updates on our project(s). You can also tweet us questions or just share blogs or stories.
[GitHub Issue](https://github.com/moov-io) | If you are able to reproduce a problem please open a GitHub Issue under the specific project that caused the error.
[moov-io slack](https://slack.moov.io/) | Join our slack channel to have an interactive discussion about the development of the project.

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
