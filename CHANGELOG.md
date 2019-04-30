## v0.4.0 (Unreleased)

ADDITIONS

- gl: add health check and verify account exists when creating an Originator
- fed: add health check and check routing numbers to verify ABA routing numbers
- transfers: support creating IAT and WEB transactions
- files: Merge and upload ACH files from incoming transfers to their SFTP destinations
- files: add basic prometheus metrics for merging/uploading
- transfers: implement and test getUserTransferFiles() and validateUserTransfer()

IMPROVEMENTS

- Support `-log.format=json` for Go Kit log formats
- transfers: handle 10 digit ImmediateOrigin values
- all: rename customer to receiver
- ofac: call 'GET /search?q=..' to also check SDNs and AltNames
  - We prefer SDNs, but the SDN of an AltName with a higher match is returned

BUG FIXES

- Fix Amount roudning errors that rarely occurred
- microDeposits: limit request body read
- http: update moov-io/base to include idempotency key checks
- all: return database/sql Rows.Err where applicable

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
