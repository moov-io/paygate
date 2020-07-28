// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package events

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path"
	"path/filepath"

	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/paygate/pkg/config"

	"gocloud.dev/pubsub"
)

func buildMessage(eventID string, event interface{}) (*pubsub.Message, error) {
	bs, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	meta := make(map[string]string)
	meta["eventID"] = eventID

	return &pubsub.Message{
		Body:     bs,
		Metadata: meta,
	}, nil
}

func buildFileURL(cfg config.Admin, filename string) (string, error) {
	u, err := url.Parse(cfg.ExternalURL)
	if err != nil {
		return "", fmt.Errorf("events: error parsing admin external url: %v", err)
	}
	if u.Host == "" {
		u.Scheme = "http"
		u.Host = "localhost"

		if _, port, _ := net.SplitHostPort(cfg.BindAddress); port != "" {
			u.Host += ":" + port
		} else {
			u.Host += bind.Admin("paygate")
		}
	}
	u.Path = path.Join(filepath.Join("inbound", filename))
	return u.String(), nil
}
