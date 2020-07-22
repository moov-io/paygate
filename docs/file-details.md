## ACH File Details

### File Header

These values are set from the `odfi.gateway` object [in the file config](https://github.com/moov-io/paygate/blob/master/docs/config.md#odfi). If those values are blank then the Origin / Destination values are set from the corresponding Account's `RoutingNumber`.

- `ImmediateOrigin`: Set from either `odfi.gateway.origin` or `odfi.routingNumber`
- `ImmediateOriginName`: Set from `odfi.gateway.originName`
- `ImmediateDestination`: Set from either `odfi.gateway.destination` or the source/destination Account `RoutingNumber`
- `ImmediateDestinationName`:  Set from `odfi.gateway.destinationName`

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
