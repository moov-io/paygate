PayGate is a RESTful API for transferring money over the Automated Clearing House (ACH) network and also functions as an ACH file shipping, merging and routing optimization service. This means that Financial Institutions (FI) can leverage PayGate to reduce cost and increase efficiency of their ACH network.

## Table of Contents

1. [Transfer Submission](#transfer-submission)
1. [File Details](#file-details)
1. [File Merging](#file-merging)
1. [Incoming Files](#incoming-files)
1. [Returned Files](#returned-files)

## Transfer Submission

As Transfers are created in PayGate [with the HTTP endpoint](https://moov-io.github.io/paygate/api/#post-/transfers) they are created by `fundflow.FirstParty` as their own ACH file and immediately written to disk under the at the path specified by the config's `storage.local.directory`. This allows each file to be manually uploaded if needed and introspection prior to upload to the ODFI's server.

The `Xfer` pair of a `Transfer` and `*ach.File` is  published on a stream (by default in-memory) to be consumed by our `XferAggregator` type. On the consuming side of that stream they're written to the local disk as an independent file which can be uploaded as-is if needed.

On each cutoff window (e.g. 5pm in New York) PayGate will gather transfers, [attempt to merge them](#merging-of-ach-files) and submit to the ODFI's server. This is done to optimize cost, latency, and easier operational verification. The submission pushes files into the larger ACH network and by default will always be NACHA compliant. Those merges files pass through transformers, which right includes an optional GPG encryption step. After they are passed through an output encoding step that could convert files to Base64, treat them as encrypted bytes, or maintain the default Nacha format. After upload the merged file is written to a `./uploaded` subdirectory after successful upload. Notifications are sent (e.g. to Email, Slack, PagerDuty) according to the success or failure of upload.

### Streaming

PayGate uses the [gocloud.dev pubsub package](https://gocloud.dev/howto/pubsub/) to have a common interface for many popular streaming services. Kafka or in-memory streams are recommended and supported. `Xfer` messages are encoded into JSON and consumed.

## File Details

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

Note: After Customers v0.5.0 we are planning to support Businesses sending Transfers which would result in alternate SEC codes.

#### Standard Entry Class Codes (SEC Codes)

- PPD: Funds transfer often for independent contractors where they have no balance - i.e. responding to an invoice for work performed.

**Future Support**

- CCD: Business funds transfer used when Transfers involve a business rather than individual person.
- WEB: Funds transfer typically related to flushing all or some of a balance in a sub ledger.

### Entry Detail

- `IndividualName`
   - On Credits this is populated from the destination Customer's `FirstName` and `LastName`.
   - On Debits this is populated from the source Customer's `FirstName` and `LastName`.

#### Addenda05

- `PaymentRelatedInformation`: This field is populated from the Transfer's `Description` field.

## File Merging

ACH transfers are merged (grouped) according their file header values using [`ach.MergeFiles`](https://godoc.org/github.com/moov-io/ach#MergeFiles). Transfers and their EntryDetail records that are merged do not modify any field. This is done primarily to reduce the fees charged by your ODFI or The Federal Reserve.

### Uploads of Merged ACH Files

ACH files which are uploaded to another FI primarily use FTP(s) ([File Transport Protocol](https://en.wikipedia.org/wiki/File_Transfer_Protocol) with TLS) or SFTP ([SSH File Transfer Protocol](https://en.wikipedia.org/wiki/SSH_File_Transfer_Protocol)) and follow a filename pattern like: `YYYYMMDD-ABA.ach` (example: `20181222-301234567.ach`). The configuration file determines how PayGate uploads and transforms the files.

### Filename templates

PayGate supports templated naming of ACH files prior to their upload. This is helpful for ODFI's which require specific naming of uploaded files. Templates use Go's [`text/template` syntax](https://golang.org/pkg/text/template/) and are validated when PayGate starts or changed via admin endpoints.

Example:

> {{ date "20060102" }}-{{ .RoutingNumber }}.ach{{ if .GPG }}.gpg{{ end }}

The following struct is passed to templates giving them data to build a filename from:

```Go
type filenameData struct {
	RoutingNumber string

	// GPG is true if the file has been encrypted with GPG
	GPG bool
}
```

Also, several functions are available (in addition to Go's standard template functions)

- `date`: Takes a Go [`Time` format](https://golang.org/pkg/time/#Time.Format) and returns the formatted string
- `env` Takes an environment variable name and returns the value from `os.Getenv`.

Note: By default filenames have sequence numbers which are incremented by PayGate and are assumed to be in a specific format. It is currently (as of 2019-10-14) undefined behavior what happens to incremented sequence numbers when filenames are in a different format. Please open issue if you run into problems here.

### IP Whitelisting

When PayGate uploads an ACH file to the ODFI server it can verify the remote server's hostname resolves to a whitelisted IP or CIDR range. This supports certain network controls to prevent DNS poisoning or misconfigured routing.

Setting `file_transfer_configs.allowed_ips` can be done with values like: `35.211.43.9` (specific IP address), `10.4.0.0/16` (CIDR range), `10.1.0.12,10.3.0.0/16` (Multiple values)

### SFTP Host and Client Key Verification

PayGate can verify the remote SFTP server's host key prior to uploading files and it can have a client key provided. Both methods assist in authenticating PayGate and the remote server prior to any file uploads.

**Public Key** (SSH Authorized key format)

```
Column: file_transfer_configs.host_public_key

Format: ssh-rsa AAAAB...wwW95ttP3pdwb7Z computer-hostname
```

**Private Key** (PKCS#8)

```
Column: file_transfer_configs.client_private_key

Format:
-----BEGIN RSA PRIVATE KEY-----
...
33QwOLPLAkEA0NNUb+z4ebVVHyvSwF5jhfJxigim+s49KuzJ1+A2RaSApGyBZiwS
...
-----END RSA PRIVATE KEY-----
```

Note: Public and Private keys can be encoded with base64 from the following formats or kept as-is. We expect Go's `base64.StdEncoding` encoding (not base64 URL encoding).

## Incoming Files

Incoming ACH files are downloaded via SFTP by PayGate and processed. Each file is expected to be an IAT file or be a NOC/COR file with an [Addenda98](https://godoc.org/github.com/moov-io/ach#Addenda98) ACH record containing a change code. This change code is used to indicate which `Customer` or `Account` fields of the file are incorrect and need changed before uploading to the ODFI's server again.

By default PayGate does not update user-created objects from these files and the `Transfer` status is updated to `FAILED` and change code saved.

## Returned Files

Returned ACH files are downloaded via SFTP by PayGate and processed. Each file is expected to have an [Addenda99](https://godoc.org/github.com/moov-io/ach#Addenda99) ACH record containing a return code. This return code is used sometimes to update the Transfer status. Transfers are always marked as `FAILED` upon their return being processed and return code saved.

The moov-io/ach documentation [includes the full set of NACHA return codes](https://moov-io.github.io/ach/returns.html). It's good to read the [Dwolla blog post on ACH returns](https://www.dwolla.com/updates/understanding-ach-returns-process/).
