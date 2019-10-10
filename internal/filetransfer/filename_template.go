// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"
)

var (
	// defaultFilenameTemplate is paygate's standard filename format for ACH files which are uploaded to an ODFI
	//
	// The format consists of a few parts: "year month day" timestamp, routing number, and sequence number
	//
	// Examples:
	//  - 20191010-987654320-1.ach
	//  - 20191010-987654320-1.ach.gpg (GPG encrypted)
	defaultFilenameTemplate = `{{ date "20060102" }}-{{ .RoutingNumber }}-{{ .N }}.ach{{ if .GPG }}.gpg{{ end }}`
)

type filenameData struct {
	RoutingNumber string
	TransferType  string

	// N is the sequence number for this file
	N string

	// GPG is true if the file has been encrypted with GPG
	GPG bool
}

var filenameFunctions template.FuncMap = map[string]interface{}{
	"date": func(pattern string) string {
		return time.Now().Format(pattern)
	},
	"env": func(name string) string {
		return os.Getenv(name)
	},
}

func renderACHFilename(raw string, data filenameData) (string, error) {
	if raw == "" {
		raw = defaultFilenameTemplate
	}
	t, err := template.New(data.RoutingNumber).Funcs(filenameFunctions).Parse(raw)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// roundSequenceNumber converts a sequence (int) to it's string value, which means 0-9 followed by A-Z
func roundSequenceNumber(seq int) string {
	if seq < 10 {
		return fmt.Sprintf("%d", seq)
	}
	// 65 is ASCII/UTF-8 value for A
	return string(65 + seq - 10) // A, B, ...
}

// achFilenameSeq returns the sequence number from a given achFilename
// A sequence number of 0 indicates an error
//
// TODO(adam): how do we do this with custom filenames?
func achFilenameSeq(filename string) int {
	parts := strings.Split(filename, "-")
	if len(parts) < 3 {
		return 0
	}
	if parts[2] >= "A" && parts[2] <= "Z" {
		return int(parts[2][0]) - 65 + 10 // A=65 in ASCII/UTF-8
	}
	n, _ := strconv.Atoi(strings.TrimSuffix(parts[2], ".ach"))
	return n
}
