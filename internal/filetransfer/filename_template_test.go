// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestFilenameTemplate(t *testing.T) {
	// default
	filename, err := renderACHFilename(defaultFilenameTemplate, filenameData{
		RoutingNumber: "987654320",
		N:             "1",
		GPG:           true,
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := fmt.Sprintf("%s-987654320-1.ach.gpg", time.Now().Format("20060102"))
	if filename != expected {
		t.Errorf("filename=%s", filename)
	}

	// example from original issue
	linden := `{{ if eq .TransferType "push" }}PS_{{ else }}PL_{{ end }}{{ date "20060102" }}.ach`
	filename, err = renderACHFilename(linden, filenameData{
		// included in template
		TransferType: "pull",
		// not included in template
		GPG:           true,
		RoutingNumber: "987654320",
	})
	if err != nil {
		t.Fatal(err)
	}

	expected = fmt.Sprintf("PL_%s.ach", time.Now().Format("20060102"))
	if filename != expected {
		t.Errorf("filename=%s", filename)
	}
}

func TestFilenameTemplate__functions(t *testing.T) {
	cases := []struct {
		tmpl, expected string
		data           filenameData
	}{
		{
			tmpl:     "static-template",
			expected: "static-template",
		},
		{
			tmpl:     `{{ env "PATH" }}`,
			expected: os.Getenv("PATH"),
		},
		{
			tmpl:     `{{ date "2006-01-02" }}`,
			expected: time.Now().Format("2006-01-02"),
		},
	}
	for i := range cases {
		res, err := renderACHFilename(cases[i].tmpl, cases[i].data)
		if err != nil {
			t.Errorf("#%d: %v", i, err)
		}
		if cases[i].expected != res {
			t.Errorf("#%d: %s", i, res)
		}
	}
}

func TestFilenameTemplate__roundSequenceNumber(t *testing.T) {
	if n := roundSequenceNumber(0); n != "0" {
		t.Errorf("got %s", n)
	}
	if n := roundSequenceNumber(10); n != "A" {
		t.Errorf("got %s", n)
	}
}

func TestFilenameTemplate__validateTemplate(t *testing.T) {
	if err := validateTemplate(defaultFilenameTemplate); err != nil {
		t.Fatal(err)
	}
	if err := validateTemplate("{{ blarg }}"); err == nil {
		t.Error("expected error")
	}
	if err := validateTemplate("{{ .Invalid }"); err == nil {
		t.Error("expected error")
	}
}

func TestFilenameTemplate__ValidateTemplates(t *testing.T) {
	if err := ValidateTemplates(newTestStaticRepository("ftp")); err != nil {
		t.Errorf("expected no error: %v", err)
	}

	repo := createTestSQLiteRepository(t)
	if err := ValidateTemplates(repo); err != nil {
		t.Errorf("no templates, didn't expect to error: %v", err)
	}

	// write a valid template and check it
	err := repo.upsertConfig(&Config{
		RoutingNumber:            "987654320",
		OutboundFilenameTemplate: `{{ date "20060102" }}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateTemplates(repo.sqlRepository); err != nil {
		t.Error(err)
	}

	// write an invalid template and check it
	err = repo.upsertConfig(&Config{
		RoutingNumber:            "123456789",
		OutboundFilenameTemplate: `{{ .Invalid }`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateTemplates(repo.sqlRepository); err == nil {
		t.Log(err)
		t.Error("expected error")
	}
}

func TestFilenameTemplate__achFilenameSeq(t *testing.T) {
	if n := achFilenameSeq("20060102-987654320-1.ach"); n != 1 {
		t.Errorf("n=%d", n)
	}
	if n := achFilenameSeq("20060102-987654320-2.ach.gpg"); n != 2 {
		t.Errorf("n=%d", n)
	}
	if n := achFilenameSeq("my-20060102-987654320-3.ach"); n != 3 {
		t.Errorf("n=%d", n)
	}
	if n := achFilenameSeq("20060102-B-987654320.ach"); n != 11 {
		t.Errorf("n=%d", n)
	}
}
