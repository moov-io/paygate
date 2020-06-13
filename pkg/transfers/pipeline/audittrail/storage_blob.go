// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package audittrail

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/internal/gpgx"
	"github.com/moov-io/paygate/pkg/config"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/output"
	"github.com/moov-io/paygate/pkg/transfers/pipeline/transform"
	"golang.org/x/crypto/openpgp"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/memblob"
	_ "gocloud.dev/blob/s3blob"
)

// blobStorage implements Storage with gocloud.dev/blob which allows
// clients to use AWS S3, GCP Storage, and Azure Storage.
type blobStorage struct {
	bucket          *blob.Bucket
	outputFormatter *output.NACHA
	pubKey          openpgp.EntityList
}

func newBlobStorage(cfg *config.AuditTrail) (*blobStorage, error) {
	storage := &blobStorage{
		outputFormatter: &output.NACHA{},
	}

	bucket, err := blob.OpenBucket(context.Background(), cfg.BucketURI)
	if err != nil {
		return nil, err
	}
	storage.bucket = bucket

	if cfg.GPG != nil {
		pubKey, err := gpgx.ReadArmoredKeyFile(cfg.GPG.KeyFile)
		if err != nil {
			return nil, err
		}
		storage.pubKey = pubKey
	}
	return storage, nil
}

func (bs *blobStorage) Close() error {
	if bs == nil {
		return nil
	}
	return bs.bucket.Close()
}

func (bs *blobStorage) SaveFile(filename string, file *ach.File) error {
	result := &transform.Result{File: file}

	var buf bytes.Buffer
	if err := bs.outputFormatter.Format(&buf, result); err != nil {
		return err
	}

	encrypted, err := gpgx.Encrypt(buf.Bytes(), bs.pubKey)
	if err != nil {
		return err
	}

	// write the file in a sub-path of the yyy-mm-dd
	path := fmt.Sprintf("audit-trail/%s/%s", time.Now().Format("2006-01-02"), filename)
	w, err := bs.bucket.NewWriter(context.Background(), path, nil)
	if err != nil {
		return err
	}

	_, copyErr := w.Write(encrypted)
	closeErr := w.Close()

	if copyErr != nil || closeErr != nil {
		return fmt.Errorf("copyErr=%v closeErr=%v", copyErr, closeErr)
	}

	return nil
}
