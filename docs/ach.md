## ACH Operations

While Paygate is featured as a RESTful API for transferring money over the Automated Clearing House (ACH) network paygate also functions as an ACH file shipping, merging and routing optimization service. This means that Financial Institutions (FI) can leverage paygate to reduce cost and increase efficiency of their ACH network.

### Transfer Submission

Paygate will periodically enqueue transfers for submission to the larger ACH network. Paygate does so by merging transfers into larger ACH files to be sent to another FI according to payment window cutoff times (e.g. 5pm in New York) or file based limitations. Typically this periodic operation occurs every 10 minutes, but can be configured with `ACH_FILE_TRANSFER_INTERVAL=5m` (for 5 minutes).

ACH files that are merged do not modify any specific transfers, but are primarily done so to reduce cost charged by the Federal Reserve for ACH payments.

### OFAC Checks

As required by United States law and NACHA guidelines all transfers are checked against the Office of Foreign Asset Control (OFAC) lists for sanctioned individuals and entities to combat fraud, terrorism and unlawful monetary transfers outside of the United States.

### Merging of ACH Files

ACH transfers are merged (grouped) according their destination routing numbers and optionally other properties (i.e. splitting credits and debits into two separate files) prior to submission.

The code paths for [merging and uploading ACH files is within `fileTransferController`](../file_transfer_async.go) and there is a Prometheus metric exposed for tracking merged ACH transfers `transfers_merged_into_ach_files{destination="..", origin=".."}`.

#### Advanced Routing

The [US Federal Reserve](https://en.wikipedia.org/wiki/Federal_Reserve_Bank) has multiple locations where we can have ACH files sent to. Some Financial Institutions optimize routing of files to allow processing during the same calendar day or to benefit from physical locality. This is done in part by the ABA routing number prefix. Paygate has long term plans to perform routing optimizations like this, but currently does no such optimization.

### SFTP Uploads of Merged ACH Files

ACH files which are uploaded to another FI primarily use SFTP (Secure File Transport Protocol) and follow a filename pattern like: `YYYYMMDD-ABA-N.ach` (example: `20181222-301234567-1.ach`). The SFTP configuration is stored within a database that Paygate controls ~~and can be modified with admin HTTP endpoints~~. (Please [comment on this GitHub issue for HTTP configuration endpoints](https://github.com/moov-io/paygate/issues/147))

There's a Prometheus metric exposed for tracking ACH file uploads `ach_files_uploaded{destination="..", origin=".."}`.

#### Configuration

Paygate currently offers a read endpoint for the configuration related to SFTP file uploads. Call `GET /configs/uploads` against the admin server (`:9092` by default) to retrieve a JSON representation of the configuration. If HTTP endpoints to update/delete configuration would be helpful please [comment or star this GitHub issue](https://github.com/moov-io/paygate/issues/147).

Otherwise, the following SQLite and MySQL tables can be configured. Insert, update or delete rows from the following:

- `cutoff_times`: Last time of each operating day for ACH file uploads to be processed.
- `file_transfer_configs`: Path configuration for inbound, outbound, and return directories on the FTP server.
- `sftp_configs`: FTP configuration for ACH file uploads (authentication and connection parameters).

Note, paygate exposes a Prometheus metric `missing_ach_file_upload_configs{routing_number="..."}` which counts how often configurations aren't found for given routing numbers. These need to be addressed by a human operator (insert the proper configurations) so files can be uploaded to a Financial Institution.

### Returned ACH Files

Returned ACH files are downloaded via SFTP by paygate and processed. Each file is expected to have an [Addenda99](https://godoc.org/github.com/moov-io/ach#Addenda99) ACH record containing a return code. This return code is used sometimes to update the Depository status. Transfers are always marked as `reclaimed` upon their return being processed.

### Incoming ACH Files

TODO(adam)
