## Encrypted Account Numbers

`Depository` account numbers are encrypted as of paygate 0.7.0 and newer. This is done as an account number is unique and sensitive to withdraw funds from an account. Federal regulations require protecting this information at rest and access to the information.

Account numbers are stored via [GoCloud CDK](https://gocloud.dev/howto/secrets/)'s Secrets ([godoc](https://godoc.org/gocloud.dev/secrets)) which can be either local or remote storage.

### Migration

On startup paygate will migrate any rows in `depositories` where the account numbers are not encrypted. This is done in batches and using the configured encryption configs.

Note: Operators should make a backup of the database prior to initiating this migration.

To view account numbers and their encrypted rows:

```
sqlite> select depository_id, user_id, account_number, account_number_encrypted, account_number_hashed  from depositories;
82..74|32..17|369090242|KTOEqJ+XbODMAebNochhVefpKz1Uz8boIfMlEaKVjdxa3FuneA+TW6fKU9eITqq7kQ==|5cdfe53c74507c13050301fcfb620966b53ad2ec7141fd1b39ce132fd3e4021b
```

Clear plaintext account numbers (after encryption migration).

```
sqlite> update depositories set account_number = '' where account_number <> '' and account_number_encrypted <> '' and account_number_hashed <> '';

sqlite> select depository_id, user_id, account_number, account_number_encrypted, account_number_hashed  from depositories;
82..74|32..17||KTOEqJ+XbODMAebNochhVefpKz1Uz8boIfMlEaKVjdxa3FuneA+TW6fKU9eITqq7kQ==|5cdfe53c74507c13050301fcfb620966b53ad2ec7141fd1b39ce132fd3e4021b
```

If you need to re-encrypt account numbers simply wipe the encrypted and hashed rows.

Note: The plaintext account numbers **must** still be in the `account_number` column to be encrypted.

```
sqlite> update depositories set account_number_encrypted = '', account_number_hashed = '' where account_number <> '' and account_number_encrypted <> '';

sqlite> select depository_id, user_id, account_number, account_number_encrypted, account_number_hashed  from depositories;
82..74|32..17|369090242||
```
