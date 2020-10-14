// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

import (
	"github.com/moov-io/ach"
)

type Direction string

const (
	Upload   Direction = "upload"
	Download Direction = "download"
)

type Message struct {
	Direction Direction
	Filename  string
	File      *ach.File
	Hostname  string
}

type Sender interface {
	Info(msg *Message) error
	Critical(msg *Message) error
}
