// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// CutoffTime represents the time of a banking day when all ACH files need to be uploaded in order
// to be processed for that day. Files which miss the cutoff time won't be processed until the next day.
//
// TODO(adam): How to handle multiple CutoffTime's for Same Day ACH?
type CutoffTime struct {
	RoutingNumber string
	Cutoff        int            // 24-hour time value (0000 to 2400)
	Loc           *time.Location // timezone cutoff is in (usually America/New_York)
}

// diff returns the time.Duration between when and the CutoffTime
// A negative value will be returned if the cutoff has already passed
func (c *CutoffTime) Diff(when time.Time) time.Duration {
	now := time.Now().In(c.Loc)
	ct := time.Date(now.Year(), now.Month(), now.Day(), c.Cutoff/100, c.Cutoff%100, 0, 0, c.Loc).In(c.Loc)
	return ct.Sub(when.In(c.Loc))
}

func (c CutoffTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		RoutingNumber string
		Cutoff        int
		Location      string
	}{
		RoutingNumber: c.RoutingNumber,
		Cutoff:        c.Cutoff,
		Location:      c.Loc.String(), // *time.Location doesn't marshal to JSON, so just write the IANA name
	})
}

func (c *CutoffTime) UnmarshalJSON(data []byte) error {
	var ct struct {
		RoutingNumber string `json:"routingNumber" yaml:"routingNumber"`
		Cutoff        int    `json:"cutoff" yaml:"cutoff"`
		Location      string `json:"location" yaml:"location"`
	}
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&ct); err != nil {
		return err
	}

	loc, err := time.LoadLocation(ct.Location)
	if err != nil {
		return err
	}

	c.RoutingNumber = ct.RoutingNumber
	c.Cutoff = ct.Cutoff
	c.Loc = loc

	return nil
}

func (c *CutoffTime) UnmarshalYAML(unmarshal func(interface{}) error) error {
	kvs := make(map[interface{}]interface{})
	err := unmarshal(&kvs)
	if err != nil {
		return err
	}
	for k, v := range kvs {
		if s, ok := k.(string); ok {
			switch s {
			case "routingNumber":
				if s, ok := v.(string); ok {
					c.RoutingNumber = s
				} else {
					return fmt.Errorf("invalid routingNumber type: %T", v)
				}
			case "cutoff":
				if n, ok := v.(int); ok {
					c.Cutoff = n
				} else {
					return fmt.Errorf("invalid cutoff type: %T", v)
				}
			case "location":
				loc, err := time.LoadLocation(v.(string))
				if err != nil {
					return fmt.Errorf("unexpected location %s: %v", v, err)
				}
				c.Loc = loc
			}
		} else {
			return fmt.Errorf("unexpected key: %v", k)
		}
	}
	return nil
}
