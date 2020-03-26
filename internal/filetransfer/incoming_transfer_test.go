// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/secrets"
	"github.com/moov-io/paygate/pkg/id"
)

func TestController__handleIncomingFile(t *testing.T) {
	controller := setupTestController(t)

	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "ppd-credit.ach"))
	if err != nil {
		t.Fatal(err)
	}

	keeper := secrets.TestStringKeeper(t)
	controller.depRepo.Depositories = []*model.Depository{
		{
			ID:            id.Depository(base.ID()),
			RoutingNumber: "076401251",
			Keeper:        keeper,
		},
	}
	controller.depRepo.Depositories[0].ReplaceAccountNumber("12345")

	req := &periodicFileOperationsRequest{skipUpload: true}

	if err := controller.handleIncomingTransfer(req, file, "to-upload.ach"); err != nil {
		t.Fatal(err)
	}
}

func TestController__handleIncomingFileErr(t *testing.T) {
	controller := setupTestController(t)
	controller.depRepo.Err = errors.New("bad error")

	file, err := parseACHFilepath(filepath.Join("..", "..", "testdata", "ppd-credit.ach"))
	if err != nil {
		t.Fatal(err)
	}

	req := &periodicFileOperationsRequest{skipUpload: true}
	if err := controller.handleIncomingTransfer(req, file, "to-upload.ach"); err != nil {
		t.Fatal(err)
	}
}
