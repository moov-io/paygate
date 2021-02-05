moov-io/paygate
===

[![GoDoc](https://godoc.org/github.com/moov-io/paygate?status.svg)](https://godoc.org/github.com/moov-io/paygate)
[![Build Status](https://github.com/moov-io/paygate/workflows/Go/badge.svg)](https://github.com/moov-io/paygate/actions)
[![Coverage Status](https://codecov.io/gh/moov-io/paygate/branch/master/graph/badge.svg)](https://codecov.io/gh/moov-io/paygate)
[![Go Report Card](https://goreportcard.com/badge/github.com/moov-io/paygate)](https://goreportcard.com/report/github.com/moov-io/paygate)
[![Apache 2 licensed](https://img.shields.io/badge/license-Apache2-blue.svg)](https://raw.githubusercontent.com/moov-io/paygate/master/LICENSE)

Moov Paygate is a RESTful HTTP API enabling Automated Clearing House ([ACH](https://en.wikipedia.org/wiki/Automated_Clearing_House)) transactions and returns to be processed with an Originating Depository Financial Institution (ODFI) without a deep understanding of a full NACHA file specification.

If you believe you have identified a security vulnerability please responsibly report the issue as via email to security@moov.io. Please do not post it to a public issue tracker.

Docs: [Project](https://moov-io.github.io/paygate/) | [API Endpoints](https://moov-io.github.io/paygate/api/) | [Admin API Endpoints](https://moov-io.github.io/paygate/admin/)

This project is sponsored by Moov Financial, Inc which offers commercial support, hosting, and additional features. Refer to our [Moov's payout documentation](https://moov.io/payouts/) for more information.

## Project Status

We've noticed that [Customers](https://github.com/moov-io/customers) and PayGate are too tightly coupled when trying to originate ACH files and are moving towards services which are better decoupled each other while offering the extensibility developers expect. We plan to make Customers read-only in the future and offer an "ACH uploader" service. On behalf of the entire Moov team, we appreciate your trust and support to build these low-level payment protocols and utilities.

## Getting Started

We publish a [public Docker image `moov/paygate`](https://hub.docker.com/r/moov/paygate/) from Docker Hub or use this repository. No configuration is required to serve on `:8083` and metrics at `:9093/metrics` in Prometheus format. We also have docker images for [OpenShift](https://quay.io/repository/moov/paygate?tab=tags).

Start PayGate and its dependencies:
```
docker-compose up
```

Create an example Transfer
```
go run examples/getting-started/main.go
```
```
Created source customer 1b1747b770bc471a478a9ae22d99973e956199aa
Customer status is verified
Created source customer account 164213fa5901431dffa324fbf16e10210fe3f6da
Approved source account
===========
Created destination customer 81393acbf9b45abc164c51689aa095528affc8e1
Customer status is verified
Created destination customer account 2a1969745050c7ccd9b2d9ba4abaf49c07c4d42e
Approved destination account
===========
{
  "transferID": "dd4dfea8ea5fe52d46d9749fc84ad6695d3d2a05",
  "amount": {
    "currency": "USD",
    "value": 125,
  },
  "source": {
    "customerID": "1b1747b770bc471a478a9ae22d99973e956199aa",
    "accountID": "164213fa5901431dffa324fbf16e10210fe3f6da"
  },
  "destination": {
    "customerID": "81393acbf9b45abc164c51689aa095528affc8e1",
    "accountID": "2a1969745050c7ccd9b2d9ba4abaf49c07c4d42e"
  },
  "description": "test transfer",
  "status": "processed",
  "sameDay": false,
  "returnCode": {
    "code": "",
    "reason": "",
    "description": ""
  },
  "created": "2020-07-10T16:59:19.6422361Z"
}
Success! A Transfer was created.

An ACH file was uploaded to a test FTP server at ./testdata/ftp-server/outbound/
total 8
-rw-r--r--  1 adam  staff  950 Jul 10 09:59 20200710-071000301.ach

```

View all uploaded files with [`achcli`](https://github.com/moov-io/ach#command-line) from [moov-io/ach](https://github.com/moov-io/ach).

```
achcli ./testdata/ftp-server/outbound/*.ach
```

<details>
<summary>Uploaded ACH file</summary>

```
Describing ACH file './testdata/ftp-server/outbound/20200710-071000301.ach'

  Origin     OriginName    Destination  DestinationName  FileCreationDate  FileCreationTime
  221475786  Teachers FCU  071000301    FRBATLANTA       200710            1259

  BatchNumber  SECCode  ServiceClassCode  CompanyName  DiscretionaryData  Identification  EntryDescription  DescriptiveDate
  1            PPD      200               John Doe                        MOOVO3S6TA      test trans        200710

    TransactionCode  RDFIIdentification  AccountNumber      Amount  Name                    TraceNumber      Category
    32               10200101            654321             125     Jane Doe                221475784797987

    TransactionCode  RDFIIdentification  AccountNumber      Amount  Name                    TraceNumber      Category
    27               22147578            123456             125     John Doe                221475784797988

  BatchCount  BlockCount  EntryAddendaCount  TotalDebitAmount  TotalCreditAmount
  1           1           2                  125               125
```

</details>

## Micro-Deposits

Micro Deposits are used to validate a customer can access an account at another Financial Institution. Typically they are two deposits under $0.50 and a balancing withdraw. The customer supplies both deposited amounts as verification.

Start PayGate and its dependencies:
```
docker-compose up
```

Setup the micro-deposit account to originate from.
```
go run examples/micro-deposits-setup/main.go
```
```
Created micro-deposit source customer df7a3f35038be2b3e332625e94b58b66fed703b8
Customer status is verified
Created customer account 460026b50830443a77253a0e6c7ca1bebae8a731
Approved micro-deposit source account
===========
In ./examples/config.yaml replace the 'validation:' YAML block with:
validation:
  microDeposits:
    source:
      customerID: "df7a3f35038be2b3e332625e94b58b66fed703b8"
      accountID: "460026b50830443a77253a0e6c7ca1bebae8a731"
===========

Restart PayGate with 'docker-compose up' and run go run /examples/micro-deposits/main.go
```

After updating PayGate's config and restarting (`docker-compose restart`)

```
go run examples/micro-deposits/main.go
```
```
Created customer 2774a3f7a5eef61dd6e2a18ac5d939dd35099161 to approve
Customer status is verified
Created destination customer account bd2b81017ecf791cdb3e722eebbe675ae67357d9
Initiating micro-deposits...
Initiated micro-deposits for destination account
Found micro-deposits: [
  {
    "currency": "USD",
    "value": 21
  },
  {
    "currency": "USD",
    "value": 2
  },
]
Customer accounts: [
  {
    "accountID": "bd2b81017ecf791cdb3e722eebbe675ae67357d9",
    "maskedAccountNumber": "**4321",
    "routingNumber": "102001017",
    "status": "validated",
    "type": "Checking"
  }
]
Success! The account was validated with micro-deposits

An ACH file was uploaded to a test FTP server at ./testdata/ftp-server/outbound/
total 8
-rw-r--r--  1 adam  staff  1900 Jul 10 10:17 20200710-071000301.ach
```

View all uploaded files with [`achcli`](https://github.com/moov-io/ach#command-line) from [moov-io/ach](https://github.com/moov-io/ach).

```
achcli ./testdata/ftp-server/outbound/*.ach
```

<details>
<summary>Uploaded ACH file</summary>

```
Describing ACH file './testdata/ftp-server/outbound/20200710-071000301.ach'

  Origin     OriginName    Destination  DestinationName  FileCreationDate  FileCreationTime
  221475786  Teachers FCU  071000301    FRBATLANTA       200710            1315

  BatchNumber  SECCode  ServiceClassCode  CompanyName     DiscretionaryData  Identification  EntryDescription  DescriptiveDate
  1            PPD      220               Micro Deposits                     MOOVO3S6TA      validation        200710

    TransactionCode  RDFIIdentification  AccountNumber      Amount  Name                    TraceNumber      Category
    22               10200101            654321             15      Jane Doe                221475787457191

  BatchNumber  SECCode  ServiceClassCode  CompanyName     DiscretionaryData  Identification  EntryDescription  DescriptiveDate
  2            PPD      220               Micro Deposits                     MOOVO3S6TA      validation        200710

    TransactionCode  RDFIIdentification  AccountNumber      Amount  Name                    TraceNumber      Category
    22               10200101            654321             25      Jane Doe                221475783240158

  BatchNumber  SECCode  ServiceClassCode  CompanyName  DiscretionaryData  Identification  EntryDescription  DescriptiveDate
  3            PPD      225               Jane Doe                        MOOVO3S6TA      validation        200710

    TransactionCode  RDFIIdentification  AccountNumber      Amount  Name                    TraceNumber      Category
    27               10200101            654321             40      Jane Doe                221475781875397

  BatchCount  BlockCount  EntryAddendaCount  TotalDebitAmount  TotalCreditAmount
  3           2           3                  40                40
```

</details>

## Next Steps

Read over our [PayGate documentation](https://moov-io.github.io/paygate/) for [configuration files](https://moov-io.github.io/paygate/config.md), api and admin endpoints, [ACH details](https://moov-io.github.io/paygate/ach) and more information.

## Getting Help

 channel | info
 ------- | -------
 [Project Documentation](https://moov-io.github.io/paygate/) | Our project documentation available online.
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
