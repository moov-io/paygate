// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func addFileCreateRoute(ww *httptest.ResponseRecorder, r *mux.Router) {
	r.Methods("POST").Path("/files/create").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		n, err := io.Copy(ww, r.Body) // write incoming body to our test ResponseRecorder
		if err != nil || n == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "fileId", "error": null}`))
	})
}

func TestFiles__createFile(t *testing.T) {
	// TODO(adam)
}

func TestFiles__CreateFile(t *testing.T) {
	w := httptest.NewRecorder()

	achClient, _, server := newACHWithClientServer("fileCreate", func(r *mux.Router) { addFileCreateRoute(w, r) })
	defer server.Close()

	id := "fileId"
	fileId, err := achClient.CreateFile("unique", &File{
		ID:              id,
		Origin:          "121042882", // Wells Fargo
		OriginName:      "my bank",
		Destination:     "231380104", // Citadel
		DestinationName: "their bank",
		Batches: []Batch{
			{
				ServiceClassCode:        200,
				StandardEntryClassCode:  "PPD",
				CompanyName:             "Your Company, Inc",
				CompanyIdentification:   "121042882",
				CompanyEntryDescription: "Online Order",
				// EffectiveEntryDate      string // defaults to today?
				ODFIIdentification: "12104288",
				EntryDetails: []EntryDetail{
					{
						TransactionCode:      22,
						RDFIIdentification:   "23138010",
						CheckDigit:           "4",
						DFIAccountNumber:     "81967038518",
						Amount:               "100000",
						IdentificationNumber: "#83738AB#      ",
						IndividualName:       "Jane Doe",
						DiscretionaryData:    "",
						TraceNumber:          "121042880000001",
						// Category:             "Forward", // TODO(adam)
						// TODO(adam): addenda
						// addendum.paymentRelatedInformation = "Bonus for working on #OSS!"
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != fileId {
		t.Errorf("id=%s fileId=%s", id, fileId)
	}

	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("got %d", w.Code)
	}

	// Decode body we sent to ACH service
	var f file
	if err := json.NewDecoder(w.Body).Decode(&f); err != nil {
		t.Fatal(err)
	}

	// Check body we sent
	if f.ID != "fileId" {
		t.Errorf("f.ID=%s", f.ID)
	}
	if f.Header.ID != "fileId" {
		t.Errorf("f.Header.ID=%v", f.Header.ID)
	}
	if len(f.Batches) != 1 {
		t.Errorf("got %d batches", len(f.Batches))
		for i := range f.Batches {
			t.Errorf("  batch[%d]=%#v", i, f.Batches[i])
		}
	}
	if f.Control.ID != "fileId" {
		t.Errorf("f.Control.ID=%v", f.Control.ID)
	}

	// Check the batch
	batch := f.Batches[0]
	if batch.Header.ID != "fileId" {
		t.Errorf("batch.Header.ID=%v", batch.Header.ID)
	}
	if len(batch.EntryDetails) != 1 {
		t.Errorf("got %d batch.EntryDetails", len(batch.EntryDetails))
		for i := range batch.EntryDetails {
			t.Errorf("  batch.EntryDetails[%d]=%#v", i, batch.EntryDetails[i])
		}
	}
	if batch.Control.ID != "fileId" {
		t.Errorf("batch.Control.ID=%v", batch.Control.ID)
	}
}
