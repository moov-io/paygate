// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/internal"
	"github.com/moov-io/paygate/internal/config"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
)

// depositoryChangeCode writes a Depository and then calls updateDepositoryFromChangeCode given the provided change code.
// The Depository is then re-read and returned from this method
func depositoryChangeCode(t *testing.T, changeCode string) *internal.Depository {
	logger := log.NewNopLogger()

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	repo := internal.NewDepositoryRepo(logger, sqliteDB.DB)

	userID := base.ID()
	dep := &internal.Depository{
		ID:       internal.DepositoryID(base.ID()),
		BankName: "my bank",
		Status:   internal.DepositoryVerified,
	}
	if err := repo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	ed := &ach.EntryDetail{
		Addenda98: &ach.Addenda98{
			CorrectedData: "", // make it non-nil
		},
	}
	cc := &ach.ChangeCode{Code: changeCode}

	if err := updateDepositoryFromChangeCode(logger, cc, ed, dep, repo); err != nil {
		t.Fatal(err)
	}

	dep, _ = repo.GetUserDepository(dep.ID, userID)
	return dep
}

func TestDepositories__updateDepositoryFromChangeCode(t *testing.T) {
	cases := []struct {
		code     string
		expected internal.DepositoryStatus
	}{
		// First Section
		{"C01", internal.DepositoryRejected},
		{"C02", internal.DepositoryRejected},
		{"C03", internal.DepositoryRejected},
		{"C04", internal.DepositoryRejected},
		{"C06", internal.DepositoryRejected},
		{"C07", internal.DepositoryRejected},
		{"C09", internal.DepositoryRejected},
		// Second Section
		{"C08", internal.DepositoryRejected},
		// Third Section // TODO(adam): these are unimplemented right now
		// {"C05", internal.DepositoryVerified},
		// {"C13", internal.DepositoryVerified},
		// {"C14", internal.DepositoryVerified},
	}
	for i := range cases {
		dep := depositoryChangeCode(t, cases[i].code)
		if dep == nil {
			t.Fatal("nil Depository")
		}
		if dep.Status != cases[i].expected {
			t.Errorf("%s: dep.Status=%v", cases[i].code, dep.Status)
		}
	}
}

func TestController__handleNOCFile(t *testing.T) {
	userID := base.ID()
	logger := log.NewNopLogger()
	dir, _ := ioutil.TempDir("", "handleNOCFile")
	defer os.RemoveAll(dir)

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	repo := newTestStaticRepository("ftp")
	depRepo := internal.NewDepositoryRepo(logger, sqliteDB.DB)

	controller, err := NewController(logger, config.Empty(), dir, repo, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// read our test file and write it into the temp dir
	fd, err := os.Open(filepath.Join("..", "..", "testdata", "cor-c01.ach"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := ach.NewReader(fd).Read()
	if err != nil {
		t.Fatal(err)
	}
	fd.Close()

	// write the Depository
	dep := &internal.Depository{
		ID: internal.DepositoryID(base.ID()),
		// fields specific to test file
		AccountNumber: strings.TrimSpace(file.Batches[0].GetEntries()[0].DFIAccountNumber),
		RoutingNumber: file.Header.ImmediateDestination,
		// other fields
		BankName:   "bank name",
		Holder:     "holder",
		HolderType: internal.Individual,
		Type:       internal.Checking,
		Status:     internal.DepositoryVerified,
		Created:    base.NewTime(time.Now().Add(-1 * time.Second)),
	}
	if err := depRepo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	// run the controller
	req := &periodicFileOperationsRequest{}
	if err := controller.handleNOCFile(req, &file, "cor-c01.ach", depRepo); err != nil {
		t.Error(err)
	}

	// check the Depository status
	dep, err = depRepo.GetUserDepository(dep.ID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if dep.Status != internal.DepositoryRejected {
		t.Errorf("dep.Status=%s", dep.Status)
	}
}

func TestController__handleNOCFileEmpty(t *testing.T) {
	logger := log.NewNopLogger()
	dir, _ := ioutil.TempDir("", "handleNOCFile")
	defer os.RemoveAll(dir)

	repo := newTestStaticRepository("ftp")

	controller, err := NewController(logger, config.Empty(), dir, repo, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// read a non-NOC file to skip its handling
	fd, err := os.Open(filepath.Join("..", "..", "testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := ach.NewReader(fd).Read()
	if err != nil {
		t.Fatal(err)
	}
	fd.Close()

	// handoff the file but watch it be skipped
	req := &periodicFileOperationsRequest{}
	if err := controller.handleNOCFile(req, &file, "ppd-debit.ach", nil); err != nil {
		t.Error(err)
	}

	// fake a NotificationOfChange array item (but it's missing Addenda98)
	file.NotificationOfChange = append(file.NotificationOfChange, file.Batches[0])
	if err := controller.handleNOCFile(req, &file, "foo.ach", nil); err != nil {
		t.Error(err)
	}
}

func TestCorrectionsErr__updateDepositoryFromChangeCode(t *testing.T) {
	userID := base.ID()
	logger := log.NewNopLogger()

	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()

	repo := internal.NewDepositoryRepo(logger, sqliteDB.DB)

	cc := &ach.ChangeCode{Code: "C14"}
	ed := &ach.EntryDetail{Addenda98: &ach.Addenda98{}}

	if err := updateDepositoryFromChangeCode(logger, cc, ed, nil, repo); err == nil {
		t.Error("nil Depository, expected error")
	} else {
		if !strings.Contains(err.Error(), "depository not found") {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// test an unexpected change code
	dep := &internal.Depository{
		ID:            internal.DepositoryID(base.ID()),
		RoutingNumber: "987654320",
		AccountNumber: "4512",
	}
	if err := repo.UpsertUserDepository(userID, dep); err != nil {
		t.Fatal(err)
	}

	// unhandled change code
	cc.Code = "C14"
	if err := updateDepositoryFromChangeCode(logger, cc, ed, dep, repo); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "unrecoverable problem with Addenda98") {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// unknown change code
	cc.Code = "C99"
	if err := updateDepositoryFromChangeCode(logger, cc, ed, dep, repo); err == nil {
		t.Error("expected error")
	} else {
		if !strings.Contains(err.Error(), "unhandled change code") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}
