// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moov-io/base/log"
	"github.com/moov-io/paygate/pkg/upload"
)

type mockedFS struct {
	fileSize int64
	modTime  time.Time
}

func (m mockedFS) Name() string {
	panic("implement me")
}

func (m mockedFS) Size() int64 {
	return m.fileSize
}

func (m mockedFS) Mode() os.FileMode {
	panic("implement me")
}

func (m mockedFS) ModTime() time.Time {
	return m.modTime
}

func (m mockedFS) IsDir() bool {
	panic("implement me")
}

func (m mockedFS) Sys() interface{} {
	panic("implement me")
}

func TestCleanupErr(t *testing.T) {
	agent := &upload.MockAgent{
		Err: errors.New("bad error"),
	}

	dir, _ := ioutil.TempDir("", "clenaup-testing")
	dl := &downloadedFiles{dir: dir}
	defer dl.deleteFiles()

	// write a test file to attempt deletion
	path := filepath.Join(dl.dir, agent.InboundPath())
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(path, "file.ach"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	// test out cleanup func
	if err := Cleanup(log.NewNopLogger(), agent, dl); err == nil {
		t.Error("expected error")
	}

	if agent.DeletedFile != "inbound/file.ach" {
		t.Errorf("unexpected deleted file: %s", agent.DeletedFile)
	}
}

func Test_CleanupEmptyFiles_InboundPath_Success(t *testing.T) {
	agent := &upload.MockAgent{}

	dir, _ := ioutil.TempDir("", "clenaup-testing")
	dl := &downloadedFiles{dir: dir}
	defer dl.deleteFiles()

	// write a test file to attempt deletion
	path := filepath.Join(dl.dir, agent.InboundPath())
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(path, "empty_file.ach"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	fileInfo := files[0]
	mockTimeNow := fileInfo.ModTime().Add(10 * time.Minute)
	minutesAfterToDelete := 5 * time.Minute

	if err := CleanupEmptyFiles(log.NewNopLogger(), agent, dl, mockTimeNow, minutesAfterToDelete); err == nil {
		t.Error("expected error")
	}

	if agent.DeletedFile != "inbound/empty_file.ach" {
		t.Errorf("unexpected deleted file: %s", agent.DeletedFile)
	}
}

func Test_CleanupEmptyFiles_ReturnPath_Success(t *testing.T) {
	agent := &upload.MockAgent{}

	dir, _ := ioutil.TempDir("", "clenaup-testing")
	dl := &downloadedFiles{dir: dir}
	defer dl.deleteFiles()

	// write a test file to attempt deletion
	path := filepath.Join(dl.dir, agent.ReturnPath())
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(path, "empty_file.ach"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	fileInfo := files[0]
	mockTimeNow := fileInfo.ModTime().Add(10 * time.Minute)
	minutesAfterToDelete := 5 * time.Minute

	if err := CleanupEmptyFiles(log.NewNopLogger(), agent, dl, mockTimeNow, minutesAfterToDelete); err == nil {
		t.Error("expected error")
	}

	if agent.DeletedFile != "return/empty_file.ach" {
		t.Errorf("unexpected deleted file: %s", agent.DeletedFile)
	}
}

func Test_CleanupEmptyFiles_Not_Run_When_Config_Is_Zero(t *testing.T) {
	agent := &upload.MockAgent{}

	dir, _ := ioutil.TempDir("", "clenaup-testing")
	dl := &downloadedFiles{dir: dir}
	defer dl.deleteFiles()

	// write a test file to attempt deletion
	path := filepath.Join(dl.dir, agent.InboundPath())
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(path, "empty_file.ach"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	fileInfo := files[0]
	mockTimeNow := fileInfo.ModTime().Add(10 * time.Minute)
	minutesAfterToDelete := 0 * time.Minute

	if err := CleanupEmptyFiles(log.NewNopLogger(), agent, dl, mockTimeNow, minutesAfterToDelete); err != nil {
		t.Error("expected nil")
	}

	if agent.DeletedFile == "inbound/empty_file.ach" {
		t.Errorf("unexpected deleted file: %s", agent.DeletedFile)
	}
}

func Test_ShouldDeleteEmptyFile_Success(t *testing.T) {
	now := time.Now()
	mfs := &mockedFS{
		fileSize: 0,
		modTime:  now.Add(time.Minute * -10),
	}
	actual := shouldDeleteEmptyFile(mfs, now, 10)
	if actual == false {
		t.Error("expected true")
	}
}

func Test_ShouldDeleteEmptyFile_Fails_File_Size_Greater_Than_Zero(t *testing.T) {
	now := time.Now()
	mfs := &mockedFS{
		fileSize: 1,
		modTime:  now.Add(time.Minute * 10),
	}
	actual := shouldDeleteEmptyFile(mfs, now, 10)
	if actual == true {
		t.Error("expected false")
	}
}

func Test_ShouldDeleteEmptyFile_Returns_False_When_File_Size_Is_Not_Zero(t *testing.T) {
	mfs := &mockedFS{
		fileSize: 12,
	}
	actual := shouldDeleteEmptyFile(mfs, time.Now(), 10)
	if actual == true {
		t.Error("expected false")
	}
}

func Test_ShouldDeleteEmptyFile_Returns_False_When_Under_Threshold(t *testing.T) {
	mfs := &mockedFS{
		fileSize: 12,
	}
	actual := shouldDeleteEmptyFile(mfs, time.Now(), 10)
	if actual == true {
		t.Error("expected false")
	}
}
