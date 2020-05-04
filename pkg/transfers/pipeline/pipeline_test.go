// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package pipeline

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPipeline__Xfer(t *testing.T) {
	body := []byte(`{"transfer":{"transferID":"0b8b19c1658cfeeadbd9a6c506c46f150796154c","amount":"USD 1.24","source":{"customerID":"foo","accountID":"foo"},"destination":{"customerID":"bar","accountID":"bar"},"description":"test payment","status":"pending","sameDay":false,"returnCode":{"code":"","reason":"","description":""},"created":"2020-05-04T13:29:18.157015-07:00"},"file":{"id":"","fileHeader":{"id":"","immediateDestination":"076401251","immediateOrigin":"076401251","fileCreationDate":"080729","fileCreationTime":"1511","fileIDModifier":"A","immediateDestinationName":"achdestname","immediateOriginName":"companyname"},"batches":[{"batchHeader":{"id":"","serviceClassCode":225,"companyName":"companyname","companyIdentification":"origid","standardEntryClassCode":"PPD","companyEntryDescription":"CHECKPAYMT","companyDescriptiveDate":"000002","effectiveEntryDate":"080730","originatorStatusCode":1,"ODFIIdentification":"07640125","batchNumber":1},"entryDetails":[{"id":"","transactionCode":27,"RDFIIdentification":"05320001","checkDigit":"9","DFIAccountNumber":"12345            ","amount":10500,"identificationNumber":"c-1            ","individualName":"Bachman Eric          ","discretionaryData":"DD","traceNumber":"076401255655291"}],"batchControl":{"id":"","serviceClassCode":225,"entryAddendaCount":1,"entryHash":5320001,"totalDebit":10500,"totalCredit":0,"companyIdentification":"origid","ODFIIdentification":"07640125","batchNumber":1}}],"IATBatches":null,"fileControl":{"id":"","batchCount":1,"blockCount":1,"entryAddendaCount":1,"entryHash":5320001,"totalDebit":10500,"totalCredit":0},"fileADVControl":{"id":"","batchCount":0,"entryAddendaCount":0,"entryHash":0,"totalDebit":0,"totalCredit":0},"NotificationOfChange":null,"ReturnEntries":null}}`)

	var xfer Xfer
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&xfer); err != nil {
		t.Fatal(err)
	}

	if xfer.Transfer.TransferID != "0b8b19c1658cfeeadbd9a6c506c46f150796154c" {
		t.Errorf("transferID=%s", xfer.Transfer.TransferID)
	}
	if xfer.File.Header.ImmediateOrigin != "076401251" {
		t.Errorf("FileHeader=%#v", xfer.File.Header)
	}
	if n := len(xfer.File.Batches); n != 1 {
		t.Errorf("got %d batches", n)
	}
	if xfer.File.Control.EntryHash != 5320001 {
		t.Errorf("EntryHash=%d", xfer.File.Control.EntryHash)
	}
}
