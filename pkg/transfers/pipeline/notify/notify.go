// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package notify

type Message struct {
	Body string
}

type Sender interface {
	Info(msg *Message) error
	Critical(msg *Message) error
}
