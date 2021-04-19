// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achx

import (
	"errors"
	"fmt"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	customers "github.com/moov-io/customers/pkg/client"
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
	ODFIRoutingNumber  string
	Gateway            config.Gateway
	FileConfig         config.FileConfig
	CutoffTimezone     *time.Location
	EffectiveEntryDate base.Time

	// CompanyIdentification is a string passed through to the Batch Header.
	// This value can be set from auth on the request and has a fallback from
	// the file config.
	// TODO(adam): Should this have another fallback of data from the Customer object?
	CompanyIdentification string
}

func ConstructFile(id string, options Options, xfer *client.Transfer, source Source, destination Destination) (*ach.File, error) {
	file, now := ach.NewFile(), time.Now().In(options.CutoffTimezone)
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

	b, err := createPPDBatch(id, options, xfer, source, destination)
	if err != nil {
		return nil, fmt.Errorf("createBatch: PPD: %v", err)
	}
	if b == nil {
		return file, errors.New("nil Batcher created")
	}
	file.AddBatch(b)

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
