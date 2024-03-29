## PayGate

**Purpose** | **[Getting Started](./README.md#getting-started)** | **[Configuration](./config.md)**

## Project Status

We’ve designed and released [achgateway for orchestrating ACH uploads](https://github.com/moov-io/achgateway) in real production scenarios. Please read [over our guide to achgateway](https://moov.io/blog/education/ach-gateway-guide/) and give us feedback on the design or your experience! We appreciate your trust and support to build these low-level payment protocols and utilities.

### Purpose

Moov Paygate is a RESTful API enabling Automated Clearing House ([ACH](https://en.wikipedia.org/wiki/Automated_Clearing_House)) transactions to be submitted and received without a deep understanding of a full NACHA file specification.

Sending payments with PayGate currently relies on a "good funds" model where funds are available prior to sending them out in terms of ACH credits. This funding model implies that only a subset of payment product flows are supported, but PayGate offers flexibility to support additional payment products.

### Table of Contents

**Running PayGate**

1. [Configuration](./config.md)
1. [API Endpoints](https://moov-io.github.io/paygate/api/)
1. [Admin Endpoints](./admin.md)
1. [ACH Details](./ach.md)

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

PayGate stores data in SQLite or MySQL given the configuration provided to track Transfers, Micro-Deposits, and other information. This is done so PayGate can be restarted or replication setup to run in an production environment.

See the [database configuration section](./config.md#database) for more information.

### Customer verification

Before money can be transferred in the United States a few conditions must be verified by PayGate according to consumer protection, fraud detection, and foreign policy actions. These regulations are outside of Moov or PayGate but must be followed by all users of the US payment methods.

To accomplish this PayGate relies on [Moov Customers](https://github.com/moov-io/customers) for tracking sensitive consumer/business information, [Moov Fed](https://github.com/moov-io/fed) for ABA routing number lookup, and [Moov Watchman](https://github.com/moov-io/watchman) for current sanctions policy restrictions.

Micro-Deposits can only be initiated for a Customer with status `ReceiveOnly` or `Verified` and an Account in the `None` status.

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
 [Project Documentation](https://moov-io.github.io/paygate/) | Our project documentation available online.
 Twitter [@moov](https://twitter.com/moov)	| You can follow Moov.io's Twitter feed to get updates on our project(s). You can also tweet us questions or just share blogs or stories.
 [GitHub Issue](https://github.com/moov-io) | If you are able to reproduce a problem please open a GitHub Issue under the specific project that caused the error.
 [moov-io slack](https://slack.moov.io/) | Join our slack channel (`#paygate`) to have an interactive discussion about the development of the project.
