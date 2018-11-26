// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/moov-io/ach"

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

func TestFiles__CreateFile(t *testing.T) {
	w := httptest.NewRecorder()

	achClient, _, server := newACHWithClientServer("fileCreate", func(r *mux.Router) { addFileCreateRoute(w, r) })
	defer server.Close()

	bs, err := ioutil.ReadFile(filepath.Join("..", "..", "testdata", "ppd-valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := ach.FileFromJson(bs)
	if err != nil {
		t.Fatal(err)
	}

	id := file.ID

	fileId, err := achClient.CreateFile("unique", file)
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
	f, err := ach.FileFromJson(w.Body.Bytes())
	if err != nil {
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
	header := batch.GetHeader()
	if header.ID != "fileId" {
		t.Errorf("Batch Header ID=%v", header.ID)
	}
	entries := batch.GetEntries()
	if len(entries) != 1 {
		t.Errorf("got %d batch EntryDetails", len(entries))
		for i := range entries {
			t.Errorf("  batch EntryDetails[%d]=%#v", i, entries[i])
		}
	}
	if batch.GetControl().ID != "fileId" {
		t.Errorf("batch Control ID=%v", batch.GetControl().ID)
	}
}
