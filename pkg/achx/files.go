// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"fmt"
	"time"

	"github.com/moov-io/ach"
	customers "github.com/moov-io/customers/client"
	"github.com/moov-io/paygate/pkg/client"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/util"
)

type Source struct {
	Customer customers.Customer
	Account  customers.Account

	// AccountNumber contains the decrypted account number from the customers service
	AccountNumber string
}

type Destination struct {
	Customer customers.Customer
	Account  customers.Account

	// AccountNumber contains the decrypted account number from the customers service
	AccountNumber string
}

type Options struct {
	ODFIRoutingNumber string
	Gateway           config.Gateway
	FileConfig        config.Transfers
}

func ConstructFile(id string, options Options, companyID string, xfer *client.Transfer, source Source, destination Destination) (*ach.File, error) {
	file, now := ach.NewFile(), time.Now()
	file.ID = id
	file.Control = ach.NewFileControl()

	// File Header
	file.Header.ID = id

	// Set origin and destination
	file.Header.ImmediateOrigin = determineOrigin(options)
	file.Header.ImmediateDestination = determineDestination(options, source, destination)

	// Set other header fields
	file.Header.ImmediateOriginName = options.Gateway.OriginName
	file.Header.ImmediateDestinationName = options.Gateway.DestinationName

	// Set file date/time from current time
	file.Header.FileCreationDate = now.Format("060102") // YYMMDD
	file.Header.FileCreationTime = now.Format("1504")   // HHMM

	// Right now we only support creating PPD files
	batch, err := createPPDBatch(id, options, companyID, xfer, source, destination)
	if err != nil {
		return nil, fmt.Errorf("constructACHFile: PPD: %v", err)
	}
	file.AddBatch(batch)

	if err := file.Create(); err != nil {
		return file, err
	}

	return file, file.Validate()
}

func determineOrigin(options Options) string {
	return util.Or(options.Gateway.Origin, options.ODFIRoutingNumber)
}

func determineDestination(options Options, src Source, dest Destination) string {
	if options.Gateway.Destination != "" {
		return options.Gateway.Destination
	}
	if options.ODFIRoutingNumber == src.Account.RoutingNumber {
		return dest.Account.RoutingNumber
	}
	return src.Account.RoutingNumber
}
