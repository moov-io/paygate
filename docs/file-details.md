## ACH File Details

### Batch Header

- `CompanyName`: This field is populated from the source Customer's `FirstName` and `LastName`.
   - Note: Businesses are being worked on to have a Name field.
- `CompanyDiscretionaryData`: This field is populated from the Metadata `discretionary` key/value pair.

### PPD

### Entry Detail

- `IndividualName`
   - On Credits this is populated from the destination Customer's `FirstName` and `LastName`.
   - On Debits this is populated from the source Customer's `FirstName` and `LastName`.

#### Addenda05

- `PaymentRelatedInformation`: This field is populated from the Transfer's `Description` field.
