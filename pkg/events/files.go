// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package events

import (
	"github.com/moov-io/ach"
	"github.com/moov-io/base"
	"github.com/moov-io/paygate/pkg/config"

	"gocloud.dev/pubsub"
)

type InboundFile struct {
	EventID   string
	EventType string
	FileURL   string
}

func CreateCorrectionFileEvent(cfg config.Admin, filename string, file *ach.File) (*pubsub.Message, error) {
	event := &InboundFile{
		EventID:   base.ID(),
		EventType: "CorrectionFile",
	}
	return finishFileEvent(cfg, event, filename)
}

func CreatePrenoteFileEvent(cfg config.Admin, filename string, file *ach.File) (*pubsub.Message, error) {
	event := &InboundFile{
		EventID:   base.ID(),
		EventType: "PrenoteFile",
	}
	return finishFileEvent(cfg, event, filename)
}

func CreateReturnFileEvent(cfg config.Admin, filename string, file *ach.File) (*pubsub.Message, error) {
	event := &InboundFile{
		EventID:   base.ID(),
		EventType: "ReturnFile",
	}
	return finishFileEvent(cfg, event, filename)
}

func finishFileEvent(cfg config.Admin, event *InboundFile, filename string) (*pubsub.Message, error) {
	fileURL, err := buildFileURL(cfg, filename)
	if err != nil {
		return nil, err
	}
	event.FileURL = fileURL

	return buildMessage(event.EventID, event)
}
