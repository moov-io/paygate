## v0.8.0 (Unreleased)

Version v0.8.0 of PayGate ... internal refactoring to cleanup code ... removed `Depository` account number migration ...

TODO(adam): write docs

ADDITIONS

- admin: Include generated Go client code and OpenAPI specification
- filetransfer: add ach_file_upload_errors for tracking ACH upload errors
- transfers: introduce basic calculations for N-day transfer limits
- transfers: store the client's real ip address on creation

IMPROVEMENTS

- filetransfer: disable `Depository` field updates from NOC's, allow via config (`UPDATE_DEPOSITORIES_FROM_CHANGE_CODE=yes`)
- filetransfer: reject related objects from COR/NOC when not auto-updating fields
- filetransfer: rename admin HTTP routes to /configs/filetransfers/*
- api: use shared Error model

BUG FIXES

- admin: fix micro-deposit return unmarshal
- filetransfer: fix partial updating of FileTransferConfig in admin HTTP routes
- filetransfer: handle nil FTPTransferAgent in Close

BUILD

- build: run sonatype-nexus-community/nancy in CI
- build: upgrade Watchman to v0.13.2
- client: generate with 'client' as pacakge name
- docker: Update golang Docker tag to v1.14

## v0.7.1 (Released 2020-01-22)

BUG FIXES

- api,client: There was a mistaken character in the OpenAPI docs `Ç` which should have been `C`.

IMPROVEMENTS

- chore(deps): update moov/ach docker tag to v1.3.0

BUILD

- build: add slack notifications
- build: upgrade golangci-lint to v1.23.1

## v0.7.0 (Released 2020-01-15)

Version v0.7.0 of Paygate adds more support for return and incoming (COR/NOC) file processing along with admin methods for updating file transfer configuration without needing to restart paygate or update the underlying database directly.

Account numbers are encrypted in a migration as part of this release. Masked versions are stored alongside their encrypted form, but the cleartext values are not wiped.

Please [read over the guide for account number encryption migration](docs/account-number-encryption.md#encrypted-account-numbers).

Also included are [filename templates](https://docs.moov.io/paygate/ach/#filename-templates) for merged ACH files uploaded to an ODFI. This allows banks with specific rules for files to be used with paygate.

KYC information is now optionally read for `Originator` and `Receiver` objects on creation. Paygate uses Moov Customers to perform validation and an OFAC check of this data.

The Moov Customers service will be used to register each Originator and Receiver (and their Depository) objects with any disclaimers required to be accepted prior to Transfer creation. OFAC searches for each customer are refreshed weekly by default with a config option to change the allowed staleness.

ADDITIONS

- internal/filetransfer: add HTTP routes for updating and deleting file transfer configs
- internal/filetransfer: allow synchronous waiting for flush routes (`/files/flush?wait`)
- main: Add 'GET /version' admin endpoint
- api,client: add return codes to Depository and Transfer HTTP responses
- filetransfer: support reading a config file for routing and FTP/SFTP configuration
- filetransfer: update Depository and Originator/Receiver objects from incoming COR entries
- depositories: add admin route for overriding status
- micro-deposits: record metrics on initate and confirmation
- micro-deposits: prevent additional attempts once we've failed too many times
- customers: Refresh OFAC searches weekly by default
- filetransfer: reject file uploads if they're not whitelisted by IP address

IMPROVEMENTS

- micro-deposits: don't require x-user-id on admin route to read
- internal/filetransfer: override String() on ftp and sftp configs to hide passwords
- micro-deposits: debit the micro-deposit from the remote account when crediting
- micro-deposits: accumulate deposit amounts for a final withdraw
- files: reverse micro-deposit transactions in Accounts, if enabled
- internal/database: micro_deposits.return_code should default to an empty string
- internal/filetransfer: support additional NACHA return codes
- internal/filetransfer: pass through x-request-id and x-user-id HTTP headers
- all: wrap http.ResponseWriter with enhanced logging and responses
- all: close out db connections where they've been missed

BUG FIXES

- internal/filetransfer: fix timezone issues in CutoffTime tests
- depositories: always set proper content-type in HTTP routes
- micro-deposits: grow mysql file_id column to store '*-micro-deposit-verify' IDs
- common: Fixed an issue in Amount.Equal()
- api,client: require x-user-id HTTP header in OpenAPI spec
- all: check sql Row.Scan errors
- transfers: expand window for EffectiveEntryDate comparison against created_at
- internal/filetransfer: micro-deposit returns only need one (Receiver) Depository
- filetransfer: write filenames with their destination, not origin
- receivers: verify an updated DefaultDepository belongs to the user
- database: cleanup goroutines for metrics reporting on shutdown

BUILD

- update Docker images for moov-io dependencies
- cmd/server: `main()` method was moved to a separate package
- internal: remove methods from public exported interface and split some code out into smaller packages
- build: flush (and wait) files in CI
- Update moov/fed Docker tag to v0.4.1
- build: update Copyright headers for 2020

## v0.6.0 (Released 2019-08-19)

Version v0.6.0 of Paygate is the largest change so far in Paygate's history. This release contains changes to support multiple ACH file upload protocols (SFTP - SSH file transfers), a more responsive local development setup, secure and validated TLS connections between paygate and all of its dependencies, versioned database migrations, and several other improvements.

Also included in this release are several admin endpoints (those bound on the `:9092` HTTP server). HTTP endpoints for reading micro-deposits made against a depository (intended for customer service and integration testing), manually initiating the merging of Transfers into files for upload to their origin/ODFI, and to read ACH file upload configs (FTP, SFTP, paths, etc..).

BREAKING CHANGES

- `sftp_configs` has been renamed to `ftp_configs` as it was incorrectly named before.
  - A new table called `sftp_configs` has been created for SSH File Transfers
  - Users need to copy data and delete the table so paygate can re-create `sftp_configs` for its new purpose.

ADDITIONS

- main: bind HTTP server with TLS if HTTPS_* variables are defined
- files: support `ACH_FILE_TRANSFER_INTERVAL=off` to disable async file operations
- http: setup http.Client's with additional root certificates
- main: override -log.format with LOG_FORMAT and -http.addr with HTTP_BIND_ADDRESS
- main: override -admin.addr with HTTP_ADMIN_BIND_ADDRESS
- files: support setting additional root certificates for FTP connections
- internal/filetransfer: initial addition of the SFTP Agent
- internal/filetransfer: add HTTP endpoints for reading SFTP configs
- internal/filetransfer: sftp: override HostKeyCallback if provided in config
- internal/filetransfer: provide env variables for FTP and SFTP connections (e.g. timeouts, concurrency)
- internal/database: use lopezator/migrator for versioned database migrations
- internal/filetransfer: sftp: read base64 encoded public/private keys
- files: read ACH_FILE_MAX_LINES to override NACHA 10k line default
- internal/filetransfer: add DEV_FILE_TRANSFER_TYPE for local dev
- internal/filetransfer: sftp: create remote outbound dir and upload into it
- files: setup a channel for manually merge and uploading of Transfers
- micro-deposits: add to ACH files for upload to the Federal Reserve
- micro-deposits: add admin route for reading micro-deposits

BUG FIXES

- transfers: set CompanyDescriptiveDate to today
- internal/database: fix confusing log from copy/paste
- depositories: fix bug where multiple fields weren't updated
- change asTransfer to copy option sub-structs
- transfers: ship YYYDetail sub-objects as pointers to copy around properly
- transfers: test (transferRequest).asTransfer(..) for all supported sub-objects
- main: cleanup ACH_FILE_STORAGE_DIR and always create directories if needed
- internal/filetransfer: ftp: change back to the previous working dir after changing
- files: stop reading and downloading files concurrently
- main: if ACH_FILE_STORAGE_DIR is our zero value use ./storage/
- common: return strconv.Atoi's error when parsing whole dollar amount
- files: upload ACH files to their origin, not destination
- internal/filetransfer: sftp: fix nil panic in readFiles

IMPROVEMENTS

- transfers: verify TEL and WEB re-occurring transfers are rejected
- files: rename 'sftp' to 'ftp' as sftp is an ssh-based file transfer protocol
- internal/filetransfer: retry file reads on "internal inconsistency" errors
- build: in local dev merge and upload way more often
- micro-deposits: unhardcode amounts and post the transaction to Accounts
- internal/filetransfer: sftp: log once (in app lifetime) about missing host_public_key
- micro-deposits: don't persist (and thus don't check) inversing debit
- build: update docker images (for tests) and download tools instead
- depositories: improve logging
- pkg/achclient: ignore 404's when deleting ACH files
- build: upgrade github.com/moov-io/base to v0.10.0

BUILD

- build: push moov/paygate:latest on 'make release-push'
- chore(deps): update moov/ofac docker tag to v0.9.0

## v0.5.1 (Released 2019-06-19)

IMPROVEMENTS

- receivers: log on receiver errors with x-request-id set

## v0.5.0 (Released 2019-06-19)

ADDITIONS

- build: Add a `docker-compose` setup for local development (and quickstart)
- transfers: add config for disabling Accounts integration (See [#126](https://github.com/moov-io/paygate/issues/126))
- transfers: support TEL ACH transfers (as Single, not re-occurring)
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
- files: Merge and upload ACH files from incoming transfers to their FTP(s) destinations
- files: add basic Prometheus metrics for merging/uploading
- transfers: implement and test getUserTransferFiles() and validateUserTransfer()
- transfers: post the transaction to GL before finishing a Transfer
- transfers: process returned files and update Transfer and Depository statuses

IMPROVEMENTS

- Support `-log.format=json` for Go Kit log formats
- transfers: handle 10 digit ImmediateOrigin values
- all: rename customer to receiver
- ofac: call 'GET /search?q=..' to also check SDNs and AltNames
  - We prefer SDNs, but the SDN of an AltName with a higher match is returned
- build: replace megacheck with staticcheck
- build: Launch [GL](https://github.com/moov-io/gl) in a container for tests
- build: switch to vendor-less build and docker image

BUG FIXES

- Fix Amount rounding errors that rarely occurred
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

- Fix memory leak in some SQLite queries.

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

- Initial implementation of endpoints with SQLite backend
- `pkg/achclient`: Initial implementation of client
- Prometheus metrics for SQLite, ping route,
- Support micro-deposits flow to Verify depositories
- transfers: delete the ACH file when we delete a transfer

## v0.0.1 (Released 2018-04-26)

Initial Release
