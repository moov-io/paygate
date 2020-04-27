// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

type ODFI struct {
	RoutingNumber string `yaml:"routing_number"`
	Gateway       Gateway
}

type Gateway struct {
	Origin          string `yaml:"origin"`
	OriginName      string `yaml:"origin_name"`
	Destination     string `yaml:"destination"`
	DestinationName string `yaml:"destination_name"`
}
