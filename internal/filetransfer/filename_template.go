// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"bytes"
	"text/template"
	"time"
)

var defaultFilenameTemplate = `{{ date "20060102" }}-{{ .RoutingNumber }}-{{ .N }}.ach`

var encrypted = defaultFilenameTemplate + `{{ if .GPG }}.gpg{{ end }}`

var lindenExample = `{{ if eq .Type "push" }}PS_{{ else }}PL_{{ end }}{{ date "20060102" }}.ach`

var funcMap template.FuncMap = map[string]interface{}{
	"date": func(pattern string) string {
		return time.Now().Format(pattern)
	},
}

func build(name, raw string) (string, error) {
	// t, err := template.New("defaultFilename").Funcs(funcMap).Parse(defaultFilenameTemplate)
	t, err := template.New(name).Funcs(funcMap).Parse(raw)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, struct {
		RoutingNumber string
		N             string
		Type          string
		GPG           bool
	}{
		RoutingNumber: "987654320",
		N:             "1",
		Type:          "push",
		GPG:           true,
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
