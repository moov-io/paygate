## Micro Deposits

micro-deposits are used as one option for verifying accounts used in ACH transactions. Account validation is an important tool for businesses who originate ACH credits and debits. Using an incorrect routing and transit number and/or account number for the recipient of an ACH transaction can cost a business both time and money.

A company sends one or two very small credit ACH transactions (and sometimes a debit to remove the money) to their customer. The customer informs the company what amount(s) were deposited to and debited from their account. This verifies the account number and ensures the customer has the ability to view the account.

### ODFI Account

The Originator Depository Financial Institution (ODFI) will have an account setup to initiate the credits and debit against the proposed remote account. The configuration is specified via `ODFI_*` environmental variables (like `ODFI_ACCOUNT_NUMBER`) and will bypass its own verification.

### Allowed Attempts

Each `Depository` has a maximum number of attempts for verification. This is set as an audit trail and to prevent endless attempts as a result of brute forcing values or overloading paygate's resources.

### Metrics

Two metrics are exported: `micro_deposits_initiated` and `micro_deposits_confirmed` which are counts of each respective action performed by clients.
