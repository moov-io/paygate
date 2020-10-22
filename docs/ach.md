PayGate is a RESTful API for transferring money over the Automated Clearing House (ACH) network and also functions as an ACH file shipping, merging and routing optimization service. This means that Financial Institutions (FI) can leverage PayGate to reduce cost and increase efficiency of their ACH network.

### Transfer Submission

PayGate will periodically enqueue transfers for submission to the larger ACH network. PayGate does so by merging transfers into larger ACH files to be sent to another FI according to payment window cutoff times (e.g. 5pm in New York) or file based limitations.

ACH files that are merged do not modify any specific transfers, but are primarily done so to reduce cost charged by the Federal Reserve for ACH payments.

### Merging of ACH Files

ACH transfers are merged (grouped) according their file header values using [`ach.MergeFiles`](https://godoc.org/github.com/moov-io/ach#MergeFiles).

### Uploads of Merged ACH Files

ACH files which are uploaded to another FI primarily use FTP(s) ([File Transport Protocol](https://en.wikipedia.org/wiki/File_Transfer_Protocol) with TLS) or SFTP ([SSH File Transfer Protocol](https://en.wikipedia.org/wiki/SSH_File_Transfer_Protocol)) and follow a filename pattern like: `YYYYMMDD-ABA.ach` (example: `20181222-301234567.ach`). The configuration file determines how PayGate uploads and transforms the files.

#### Filename templates

PayGate supports templated naming of ACH files prior to their upload. This is helpful for ODFI's which require specific naming of uploaded files. Templates use Go's [`text/template` syntax](https://golang.org/pkg/text/template/) and are validated when PayGate starts or changed via admin endpoints.

Example:

```
{{ date "20060102" }}-{{ .RoutingNumber }}.ach{{ if .GPG }}.gpg{{ end }}
```

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

#### IP Whitelisting

When PayGate uploads an ACH file to the ODFI server it can verify the remote server's hostname resolves to a whitelisted IP or CIDR range. This supports certain network controls to prevent DNS poisoning or misconfigured routing.

Setting `file_transfer_configs.allowed_ips` can be done with values like: `35.211.43.9` (specific IP address), `10.4.0.0/16` (CIDR range), `10.1.0.12,10.3.0.0/16` (Multiple values)

#### SFTP Host and Client Key Verification

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

### Incoming ACH Files

Incoming ACH files are downloaded via SFTP by PayGate and processed. Each file is expected to be an IAT file or be a NOC/COR file with an [Addenda98](https://godoc.org/github.com/moov-io/ach#Addenda98) ACH record containing a change code. This change code is used to indicate which `Customer` or `Account` fields of the file are incorrect and need changed before uploading to the ODFI's server again.

By default PayGate does not update user-created objects from these files and the `Transfer` status is updated to `FAILED` and change code saved.

### Returned ACH Files

Returned ACH files are downloaded via SFTP by PayGate and processed. Each file is expected to have an [Addenda99](https://godoc.org/github.com/moov-io/ach#Addenda99) ACH record containing a return code. This return code is used sometimes to update the Transfer status. Transfers are always marked as `FAILED` upon their return being processed and return code saved.

The moov-io/ach documentation [includes the full set of NACHA return codes](https://moov-io.github.io/ach/returns.html). It's good to read the [Dwolla blog post on ACH returns](https://www.dwolla.com/updates/understanding-ach-returns-process/).
