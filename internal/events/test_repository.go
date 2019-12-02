// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package events

import (
	"github.com/moov-io/paygate/pkg/id"
)

type TestRepository struct {
	Err   error
	Event *Event
}

func (r *TestRepository) GetEvent(eventID EventID, userID id.User) (*Event, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return r.Event, nil
}

func (r *TestRepository) GetUserEvents(userID id.User) ([]*Event, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if r.Event != nil {
		return []*Event{r.Event}, nil
	}
	return nil, nil
}

func (r *TestRepository) WriteEvent(userID id.User, event *Event) error {
	return r.Err
}

func (r *TestRepository) GetUserEventsByMetadata(userID id.User, metadata map[string]string) ([]*Event, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	if r.Event != nil {
		return []*Event{r.Event}, nil
	}
	return nil, nil
}
