// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"time"
)

type File struct {
	ID              string
	Origin          string
	OriginName      string
	Destination     string
	DestinationName string
	Batches         []Batch
	// IATBatches []*IATBatch // TODO(adam)
}

type Batch struct {
	ServiceClassCode        int
	StandardEntryClassCode  string
	CompanyName             string
	CompanyIdentification   string
	CompanyEntryDescription string
	EffectiveEntryDate      time.Time
	ODFIIdentification      string
	EntryDetails            []EntryDetail
}

type EntryDetail struct {
	TransactionCode      int
	RDFIIdentification   string
	CheckDigit           string
	DFIAccountNumber     string
	Amount               string
	IdentificationNumber string
	IndividualName       string
	DiscretionaryData    string
	TraceNumber          string
	// Addenda TODO(adam)
}

func createFile(f *File) *file {
	now := time.Now()
	out := &file{
		ID: f.ID,
		Header: fileHeader{
			ID:                       f.ID,
			ImmediateOrigin:          f.Origin,
			ImmediateOriginName:      f.OriginName,
			ImmediateDestination:     f.Destination,
			ImmediateDestinationName: f.DestinationName,
			FileCreationDate:         now,
			FileCreationTime:         now,
		},
		Control: fileControl{
			ID:                f.ID,
			BatchCount:        len(f.Batches), // TODO(adam)
			BlockCount:        len(f.Batches), // TODO(adam)
			EntryAddendaCount: 0,              // TODO(adam)
		},
	}
	// Add each Batch
	for i := range f.Batches {
		batch := batch{
			Header: batchHeader{
				ID:                      f.ID,
				ServiceClassCode:        f.Batches[i].ServiceClassCode,
				StandardEntryClassCode:  f.Batches[i].StandardEntryClassCode,
				CompanyName:             f.Batches[i].CompanyName,
				CompanyIdentification:   f.Batches[i].CompanyIdentification,
				CompanyEntryDescription: f.Batches[i].CompanyEntryDescription,
				EffectiveEntryDate:      f.Batches[i].EffectiveEntryDate,
				ODFIIdentification:      f.Batches[i].ODFIIdentification,
				BatchNumber:             i,
				// CompanyDiscretionaryData, // TODO(adam)
				// CompanyDescriptiveDate    //	TODO(adam)
			},
			Control: batchControl{
				ID:                    f.ID,
				ServiceClassCode:      f.Batches[i].ServiceClassCode,
				EntryAddendaCount:     0, // TODO(adam)
				CompanyIdentification: f.Batches[i].CompanyIdentification,
				ODFIIdentification:    f.Batches[i].ODFIIdentification,
				BatchNumber:           i,
			},
		}
		for j := range f.Batches[i].EntryDetails {
			ed := f.Batches[i].EntryDetails[j]
			amt, err := strconv.ParseFloat(ed.Amount, 32)
			if err != nil {
				panic(err) // TODO(adam)
			}
			batch.EntryDetails = append(batch.EntryDetails, entryDetail{
				ID:                   f.ID,
				TransactionCode:      ed.TransactionCode,
				RDFIIdentification:   ed.RDFIIdentification,
				CheckDigit:           ed.CheckDigit,
				DFIAccountNumber:     ed.DFIAccountNumber,
				Amount:               int(amt * 100), // TODO(adam): ACH service should accept our Amount struct as a string
				IdentificationNumber: ed.IdentificationNumber,
				IndividualName:       ed.IndividualName,
				DiscretionaryData:    ed.DiscretionaryData,
				TraceNumber:          ed.TraceNumber,
				Category:             "Forward",
				// AddendaRecordIndicator:
			})
		}
		out.Batches = append(out.Batches, batch)
	}
	return out
}

type file struct {
	ID         string      `json:"id"`
	Header     fileHeader  `json:"fileHeader"`
	Batches    []batch     `json:"batches"`
	IATBatches []iatbatch  `json:"IATBatches"`
	Control    fileControl `json:"fileControl"`
}

type fileHeader struct {
	ID                       string    `json:"id"`
	ImmediateOrigin          string    `json:"immediateOrigin"`
	ImmediateOriginName      string    `json:"immediateOriginName"`
	ImmediateDestination     string    `json:"immediateDestination"`
	ImmediateDestinationName string    `json:"immediateDestinationName"`
	FileCreationDate         time.Time `json:"fileCreationDate"`
	FileCreationTime         time.Time `json:"fileCreationTime"`
}

type fileControl struct {
	ID                string `json:"id"`
	BatchCount        int    `json:"batchCount"`
	BlockCount        int    `json:"blockCount,omitempty"`
	EntryAddendaCount int    `json:"entryAddendaCount"`
	// EntryHash int `json:"entryHash"`
	// TotalDebitEntryDollarAmountInFile int `json:"totalDebit"`
	// TotalCreditEntryDollarAmountInFile int `json:"totalCredit"`
}

type batch struct {
	Header       batchHeader   `json:"batchHeader,omitempty"`
	EntryDetails []entryDetail `json:"entryDetails,omitempty"`
	Control      batchControl  `json:"batchControl,omitempty"`
}

type iatbatch struct{}

type batchHeader struct {
	ID                      string    `json:"id"`
	ServiceClassCode        int       `json:"serviceClassCode"`
	CompanyName             string    `json:"companyName"`
	CompanyIdentification   string    `json:"companyIdentification"`
	StandardEntryClassCode  string    `json:"standardEntryClassCode,omitempty"`
	CompanyEntryDescription string    `json:"companyEntryDescription,omitempty"`
	CompanyDescriptiveDate  string    `json:"companyDescriptiveDate,omitempty"`
	EffectiveEntryDate      time.Time `json:"effectiveEntryDate,omitempty"`
	ODFIIdentification      string    `json:"ODFIIdentification"`
	BatchNumber             int       `json:"batchNumber,omitempty"`
	// CompanyDiscretionaryData string `json:"companyDiscretionaryData,omitempty"`
}

type batchControl struct {
	ID                        string `json:"id"`
	ServiceClassCode          int    `json:"serviceClassCode"`
	EntryAddendaCount         int    `json:"entryAddendaÇount"`
	CompanyIdentification     string `json:"companyIdentification"`
	MessageAuthenticationCode string `json:"messageAuthentication,omitempty"`
	ODFIIdentification        string `json:"ODFIIdentification"`
	BatchNumber               int    `json:"batchNumber"`
	// EntryHash int `json:"entryHash"`
	// TotalDebitEntryDollarAmount int `json:"totalDebit"`
	// TotalCreditEntryDollarAmount int `json:"totalCredit"`
}

type entryDetail struct {
	ID                     string `json:"id"`
	TransactionCode        int    `json:"transactionCode"`
	RDFIIdentification     string `json:"RDFIIdentification"`
	CheckDigit             string `json:"checkDigit"`
	DFIAccountNumber       string `json:"DFIAccountNumber"`
	Amount                 int    `json:"amount"`
	IdentificationNumber   string `json:"identificationNumber,omitempty"`
	IndividualName         string `json:"individualName"`
	DiscretionaryData      string `json:"discretionaryData,omitempty"`
	AddendaRecordIndicator int    `json:"addendaRecordIndicator,omitempty"`
	TraceNumber            string `json:"traceNumber,omitempty"`
	Category               string `json:"category,omitempty"`
	// Addendum []Addendumer `json:"addendum,omitempty"`
}

type createFileResponse struct {
	ID    string `json:"id"`
	Error error  `json:"error"`
}

// CreateFile makes HTTP requests to our ACH service in order to create an ACH File.
//
// These Files have many fields associated, but this method performs no validation. However, the
// ACH service might return an error that callers should check.
//
// TOOD(adam): We need to save fileId in the transfers table
func (a *ACH) CreateFile(idempotencyKey string, req *File) (string, error) {
	f := createFile(req)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&f); err != nil || buf.Len() == 0 {
		return "", fmt.Errorf("CreateFile: file ID %s json encoding error: %v", req.ID, err)
	}

	resp, err := a.POST("/files/create", idempotencyKey, ioutil.NopCloser(&buf))
	if err != nil {
		return "", fmt.Errorf("CreateFile: error file ID %s : %v", f.ID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("CreateFile: file ID %s got %d HTTP status", f.ID, resp.StatusCode)
	}

	// Read response
	var response createFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("CreateFile: problem reading response: %v", err)
	}
	return response.ID, response.Error
}

type validateFileResponse struct {
	Err string `json:"error"`
}

// ValidateFile makes an HTTP request to our ACH service which performs checks on the
// file to ensure correctness.
func (a *ACH) ValidateFile(fileId string) error {
	resp, err := a.GET(fmt.Sprintf("/files/%s/validate", fileId))
	defer resp.Body.Close()

	// Try reading error from ACH service
	var response validateFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("ValidateFile: problem reading response json: %v", err)
	}
	if response.Err != "" {
		return fmt.Errorf("ValidateFile (fileId=%s): %s", fileId, response.Err)
	}

	// Just return the a.GET error
	if err != nil {
		return fmt.Errorf("ValidateFile: error making HTTP request: %v", err)
	}
	return nil
}

// GetFileContents makes an HTTP request to our ACH service and returns the plaintext ACH file.
func (a *ACH) GetFileContents(fileId string) (*bytes.Buffer, error) {
	resp, err := a.GET(fmt.Sprintf("/files/%s/contents", fileId))
	if err != nil {
		return nil, fmt.Errorf("GetFileContents: error making HTTP request: %v", err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	n, err := io.Copy(&buf, resp.Body)
	if err != nil || n == 0 {
		return nil, fmt.Errorf("GetFileContents: problem reading body (n=%d): %v", n, err)
	}
	return &buf, nil
}
