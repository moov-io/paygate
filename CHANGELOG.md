## v0.3.0 (Unreleased)

ADDITIONS

- Query Moov's [OFAC Service](https://github.com/moov-io/ofac) for Customer, Depository, and Originator information
- Save ACH file ID's for micro-deposits
- admin: Added `/live` and `/ready` endpoints for liveness and readiness and checks (Health Checks)

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
