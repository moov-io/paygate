## v0.6.1 (Released 2019-08-29)

ADDITIONS

- client: generate go code


BUG FIXES

- internal/filetransfer: fix timezone issues in CutoffTime tests
- micro-deposits: don't require x-user-id on admin route to read
- micro-deposits: grow mysql file_id column to store '*-micro-deposit-verify' IDs

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
