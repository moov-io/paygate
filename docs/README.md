## PayGate

**Purpose** | **[Getting Started](../README.md#getting-started)** | **[Configuration](./config.md)**

### Purpose

Moov Paygate is a RESTful API enabling Automated Clearing House ([ACH](https://en.wikipedia.org/wiki/Automated_Clearing_House)) transactions to be submitted and received without a deep understanding of a full NACHA file specification.

### Table of Contents

**Running PayGate**

1. [Configuration](./config.md)
1. [API Endpoints](https://moov-io.github.io/paygate/)
1. [Admin Endpoints](./admin.md)
1. [ACH Details](./ach.md)
   1. [File Details](./file-details.md)

**Dependencies**

1. [Customers](./customers.md)

**Production Checklist**

1. [Running in Production](./production.md)

**Extending PayGate**

1. [Local Development](./local-dev.md)
1. [High Availability](./ha.md)
1. [Prometheus Metrics](./metrics.md)

## Components

Below is a summary of the major components within PayGate that coordinate for ACH origination and processing.

### Database

PayGate stores data in SQLite or MySQL given the configuration provided to track Transfers, Organizations, Tenants, and other information. This is done so PayGate can be restarted or replication setup to run in an production environment.

See the [database configuration section](./config.md#database) for more information.

### Customer verification

Before money can be transferred in the United States a few conditions must be verified by PayGate according to consumer protection, fraud detection, and foreign policy actions. These regulations are outside of Moov or PayGate but must be followed by all users of the US payment methods.

To accomplish this PayGate relies on [Moov Customers](https://github.com/moov-io/customers) for tracking sensitive consumer/business information, [Moov Fed](https://github.com/moov-io/fed) for ABA routing number lookup, and [Moov Watchman](https://github.com/moov-io/watchman) for current sanctions policy restrictions.

Micro-Deposits can only be initiated for a Customer in the `Unknown` status and an Account in the `None` status.

See the [customer configuration section](./config.md#customers) for more information.

### Transfer Pipeline

Transfers can only be created if they contain valid fields, an appropriate source and destination, and configuration for your ODFI. Even if they're valid Transfers might be rejected prior to upload after being received by the other Financial Institution. After Transfer objects are created in PayGate they are pushed into a pipeline to be processed through a variety of operations to transform, merge, optimize and possibly encrypt them prior to upload to the ODFI.

The pipeline consists of several steps: audit recording, merging into ACH files, and entry/file balancing. These steps are all optional and by default PayGate will merge Transfers into as few ACH files as possible without balancing. See the [transfer pipeline configuration section](./config.md#pipeline) for more information.

PayGate is configured with cutoff windows which are timestamps to flush pending inbound and outbound files with the ODFI. There are typically several cutoff windows every banking day and are used to have payments complete faster. The Federal Reserve and financial institutions all across the US are working to increase the number of cutoff windows each banking day.

Uploading of ACH files consists of a couple steps: encryption, output formatting, and notifications on success or failure. These are all optional and ACH files will be uploaded in their clear-text NACHA format by default. See the [ODFI configuration section](./config.md#odfi) for more information.

### FTP / SFTP

PayGate interacts with FTP and SFTP (SSH File Transport Protocol) to upload and download files from an ODFI's server. Both protocols can leverage current industry standards for encryption and security.

## Getting Help

 channel | info
 ------- | -------
 [Project Documentation](https://github.com/moov-io/paygate/tree/master/docs/) | Our project documentation available online.
 [Hosted Documentation](https://docs.moov.io/paygate/) | Hosted documentation for enterprise solutions.
 Google Group [moov-users](https://groups.google.com/forum/#!forum/moov-users)| The Moov users Google group is for contributors other people contributing to the Moov project. You can join them without a google account by sending an email to [moov-users+subscribe@googlegroups.com](mailto:moov-users+subscribe@googlegroups.com). After receiving the join-request message, you can simply reply to that to confirm the subscription.
Twitter [@moov_io](https://twitter.com/moov_io)	| You can follow Moov.IO's Twitter feed to get updates on our project(s). You can also tweet us questions or just share blogs or stories.
[GitHub Issue](https://github.com/moov-io) | If you are able to reproduce a problem please open a GitHub Issue under the specific project that caused the error.
[moov-io slack](https://slack.moov.io/) | Join our slack channel (`#paygate`) to have an interactive discussion about the development of the project.
