## v0.6.0 (Unreleased)

ADDITIONS

- files: support `ACH_FILE_TRANSFER_INTERVAL=off` to disable async file operations
- http: setup http.Client's with additional root certificates
- main: override -log.format with LOG_FORMAT and -http.addr with HTTP_BIND_ADDRESS

IMPROVEMENTS

- transfers: verify TEL and WEB reoccurring transfers are rejected
- docs: trim up deploy steps and refer it as 'Getting Started'

BUG FIXES

- internal/database: check driver error types for unique violations
- transfers: set CompanyDescriptiveDate to today
- internal/database: fix confusing log from copy/paste
- depositories: fix bug where multiple fields weren't updated

BUILD

- chore(deps): update moov/ach docker tag to v1.0.2
- build: push moov/paygate:latest on 'make release-push'

## v0.5.1 (Released 2019-06-19)

IMPROVEMENTS

- receivers: log on receiver errors with x-request-id set

## v0.5.0 (Released 2019-06-19)

ADDITIONS

- build: Add a `docker-compose` setup for local development (and quickstart)
- transfers: add config for disabling Accounts integration (See [#126](https://github.com/moov-io/paygate/issues/126))
- transfers: support TEL ACH transfers (as Single, not reoccurring)
- transfers: support CCD and their required Addenda05
- files: reverse a Transfer's transaction when the ACH file has been returned
- files: add missing_ach_file_upload_configs for when configurations aren't found

IMPROVEMENTS

- receivers: pass emails through net/mail.ParseAddress
- build: update moov/fsftp to v0.2.0
- docs/ach: add examples for cutoff_times table
- accounts: close dockertest deployment after success

BUG FIXES

- transfers: test we can read an array of []transferRequest objects
- transfers: tombstone Transfer before attempting to delete from ACH service
- transfers: ignore error if a Transfer had no file_id on deletion

## v0.4.1 (Released 2019-05-20)

BUILD

- Fix build steps to publish Linux and macOS binaries

## v0.4.0 (Released 2019-05-20)

BREAKING CHANGES

- all: Changed `gl` over to `accounts` after that project was renamed

ADDITIONS

- gl: add health check and verify account exists when creating an Originator
- fed: add health check and check routing numbers to verify ABA routing numbers
- transfers: support creating IAT and WEB transactions
- files: Merge and upload ACH files from incoming transfers to their SFTP destinations
- files: add basic prometheus metrics for merging/uploading
- transfers: implement and test getUserTransferFiles() and validateUserTransfer()
- transfers: post the transaction to GL before finishing a Transfer
- transfers: process returned files and update Transfer and Depository statuses

IMPROVEMENTS

- Support `-log.format=json` for Go Kit log formats
- transfers: handle 10 digit ImmediateOrigin values
- all: rename customer to receiver
- ofac: call 'GET /search?q=..' to also check SDNs and AltNames
  - We prefer SDNs, but the SDN of an AltName with a higher match is returned
- build: repalce megacheck with staticcheck
- build: Launch [GL](https://github.com/moov-io/gl) in a container for tests
- build: switch to vendor-less build and docker image

BUG FIXES

- Fix Amount roudning errors that rarely occurred
- microDeposits: limit request body read
- http: update moov-io/base to include idempotency key checks
- all: return database/sql Rows.Err where applicable
- all: initiate sql rollbacks and log their optional error
- depositories: require validation again after AccountNumber or RoutingNumber is upserted

## v0.3.0 (Released 2019-03-08)

ADDITIONS

- Query Moov's [OFAC Service](https://github.com/moov-io/ofac) for Customer, Depository, and Originator information
- Save ACH file ID's for micro-deposits
- admin: Added `/live` and `/ready` endpoints for liveness and readiness and checks (Health Checks)
- Delete micro-deposit files after creation
  - This is a temporary workaround for the demo. This behavior is expected to change over time.

IMPROVEMENTS

- ofac: lower match threshold to 90%

BUG FIXES

- http: add CORS headers to 'GET /ping'

BUILD

- Update to Go 1.12

## v0.2.1 (Released 2019-01-16)

BUG FIXES

- Fix memory leak in some SQlite queries.

## v0.2.0 (Released 2019-01-04)

BREAKING CHANGES

- Convert to using `base.Time` from [github.com/moov-io/base](https://github.com/moov-io/base).

BUG FIXES

- Calculate EffectiveEntryDate with `base.Time` (was [github.com/moov-io/banktime](https://github.com/moov-io/banktime)). (See: [#43](https://github.com/moov-io/paygate/pull/43))

IMPROVEMENTS

- Return JSON unmarshal errors with more detail to clients.
- Validate RoutingNumber on all structs that have one.
- Create ACH files for each micro-transaction.

## v0.1.0 (Released 2018-12-14)

ADDITIONS

- Initial implementation of endpoints with sqlite backend
- `pkg/achclient`: Initial implementation of client
- Prometheus metrics for sqlite, ping route,
- Support micro-deposits flow to Verify depositories
- transfers: delete the ACH file when we delete a transfer

## v0.0.1 (Released 2018-04-26)

Initial Release
