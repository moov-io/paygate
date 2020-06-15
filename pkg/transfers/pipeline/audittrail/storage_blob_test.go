// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package audittrail

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/config"
)

var (
	keyPath = filepath.Join("..", "..", "..", "..", "internal", "gpgx", "testdata", "moov.pub")
	ppdPath = filepath.Join("..", "..", "..", "..", "testdata", "ppd-debit.ach")
)

func TestBlobStorage(t *testing.T) {
	cfg := &config.AuditTrail{
		BucketURI: "mem://",
		GPG: &config.GPG{
			KeyFile: keyPath,
		},
	}
	store, err := newBlobStorage(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	file, err := ach.ReadFile(ppdPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SaveFile("saved.ach", file); err != nil {
		t.Fatal(err)
	}

	path := fmt.Sprintf("audit-trail/%s/saved.ach", time.Now().Format("2006-01-02"))
	r, err := store.bucket.NewReader(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	bs, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(bs, []byte("BEGIN PGP MESSAGE")) {
		t.Errorf("unexpected blob\n%s", string(bs))
	}
}

func TestBlobStorageErr(t *testing.T) {
	cfg := &config.AuditTrail{
		BucketURI: "bad://",
	}
	if _, err := NewStorage(cfg); err == nil {
		t.Error("expected error")
	}
}
