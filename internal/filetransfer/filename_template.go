// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
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
func achFilenameSeq(filename string) int {
	replacer := strings.NewReplacer(".ach", "", ".gpg", "")
	parts := strings.Split(replacer.Replace(filename), "-")
	for i := range parts {
		if parts[i] >= "A" && parts[i] <= "Z" {
			return int(parts[i][0]) - 65 + 10 // A=65 in ASCII/UTF-8
		}
		// Assume routing numbers could be a minimum of 010,000,000
		// and a number is a sequence number which we can increment
		if n, err := strconv.Atoi(parts[i]); err == nil && (n > 0 && n < 10000000) {
			return n
		}
	}
	return 0
}

func ValidateTemplates(repo Repository) error {
	if r, ok := repo.(*sqlRepository); ok {
		templates, err := r.getOutboundFilenameTemplates()
		if err != nil {
			return fmt.Errorf("ValidateTemplates: %v", err)
		}
		for i := range templates {
			if err := validateTemplate(templates[i]); err != nil {
				return fmt.Errorf("ValidateTemplates: error parsing:\n  %s\n  %v", templates[i], err)
			}
		}
	}
	// If we are use another type of repository (which right now is *staticRepository)
	// just validate the default template as that'll be the only one used.
	return validateTemplate(defaultFilenameTemplate)
}

func validateTemplate(tmpl string) error {
	// create a random name for this template
	n, err := rand.Int(rand.Reader, big.NewInt(1024*1024*1024))
	if err != nil {
		return err
	}

	_, err = template.New(fmt.Sprintf("validate-%d", n)).Funcs(filenameFunctions).Parse(tmpl)
	return err
}
