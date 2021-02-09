// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"
)

type FilenameData struct {
	RoutingNumber string

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

func RenderACHFilename(raw string, data FilenameData) (string, error) {
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

// RoundSequenceNumber converts a sequence (int) to it's string value, which means 0-9 followed by A-Z
func RoundSequenceNumber(seq int) string {
	if seq < 10 {
		return fmt.Sprintf("%d", seq)
	}
	// 65 is ASCII/UTF-8 value for A
	return string(rune(65 + seq - 10)) // A, B, ...
}

// achFilenameSeq returns the sequence number from a given achFilename
// A sequence number of 0 indicates an error
func ACHFilenameSeq(filename string) int {
	replacer := strings.NewReplacer(".ach", "", ".gpg", "")
	parts := strings.Split(replacer.Replace(filename), "-")

	// Traverse the filename from right to left looking for the sequence number.
	// We assume the sequence number will be on the right side of the filename.
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] >= "A" && parts[i] <= "Z" {
			return int(parts[i][0]) - 65 + 10 // A=65 in ASCII/UTF-8
		}
		// Assume routing numbers could be a minimum of 100,000,000
		// and a number is a sequence number which we can increment
		if n, err := strconv.Atoi(parts[i]); err == nil && (n > 0 && n < 10000000) {
			return n
		}
	}
	return 0
}
