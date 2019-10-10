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
