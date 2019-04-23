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

TODO(adam): Talk about optimizing Fed bank for a transfer (based on ABA prefix)

### SFTP Uploads of Merged ACH Files

ACH files which are uploaded to another FI primarily use SFTP (Secure File Transport Protocol) and follow a filename pattern like: `YYYYMMDD-ABA-N.ach` (example: `20181222-301234567-1.ach`). Tne SFTP configuration is stored within a database that Paygate controls and can be modified with admin HTTP endpoints.

There's a Prometheus metric exposed for tracking ACH file uploads `ach_files_uploaded{destination="..", origin=".."}`.

#### Configuration

TODO(adam): admin HTTP endpoints

### Returned ACH Files

TODO(adam): Addenda99 (ReturnCode), dedup matching, Depository and Transfer status updates

### Incoming ACH Files

TODO(adam)
