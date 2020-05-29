// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/moov-io/ach"
	"github.com/moov-io/paygate/pkg/config"
)

type Email struct {
	// smtp client
	cfg *config.Email
}

type EmailTemplateData struct {
	CompanyName string // e.g. Moov
	Verb        string // e.g. uploaded, downloaded
	Filename    string // e.g. 20200529-131400.ach

	DebitTotal  int
	CreditTotal int

	BatchCount int
	EntryCount int
}

var (
	// Ensure the default template validates against our data struct
	_ = config.DefaultEmailTemplate.Execute(ioutil.Discard, EmailTemplateData{})
)

func NewEmail(cfg *config.Email) (*Email, error) {
	return &Email{
		cfg: cfg,
	}, nil
}

// use msg.File and msg.Filename with template to marshal email contents

func (mailer *Email) Info(msg *Message) error {
	contents, err := marshalEmail(mailer.cfg, msg)
	if err != nil {
		return err
	}
	fmt.Printf("[INFO] email: contents=%s", contents)
	return nil
}

func (mailer *Email) Critical(msg *Message) error {
	fmt.Printf("[CRITICAL] email: message=%#v", msg)
	return nil
}

func marshalEmail(cfg *config.Email, msg *Message) (string, error) {
	data := EmailTemplateData{
		CompanyName: cfg.CompanyName,
		Verb:        string(msg.Direction),
		Filename:    msg.Filename,
		DebitTotal:  msg.File.Control.TotalDebitEntryDollarAmountInFile,
		CreditTotal: msg.File.Control.TotalCreditEntryDollarAmountInFile,
		BatchCount:  msg.File.Control.BatchCount,
		EntryCount:  countEntries(msg.File),
	}

	var buf bytes.Buffer
	if err := cfg.Tmpl().Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func sendEmail() error {
	return nil
}

func countEntries(file *ach.File) int {
	var total int
	for i := range file.Batches {
		total += len(file.Batches[i].GetEntries())
	}
	return total
}
