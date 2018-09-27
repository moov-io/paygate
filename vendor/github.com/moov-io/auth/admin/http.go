// Copyright 2018 The ACH Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"context"
	"net/http"
	"time"
)

func NewServer() *Server {
	timeout, _ := time.ParseDuration("45s")
	return &Server{
		svc: &http.Server{
			Addr:         ":9090",
			Handler:      Handler(),
			ReadTimeout:  timeout,
			WriteTimeout: timeout,
			IdleTimeout:  timeout,
		},
	}
}

// Server represents a holder around a net/http Server which
// is used for admin endpoints. (i.e. metrics, healthcheck)
type Server struct {
	svc *http.Server
}

func (s *Server) BindAddr() string {
	return s.svc.Addr
}

// Start brings up the admin HTTP service. This call blocks.
func (s *Server) Listen() error {
	if s == nil || s.svc == nil {
		return nil
	}
	return s.svc.ListenAndServe()
}

// Shutdown unbinds the HTTP server.
func (s *Server) Shutdown() {
	if s == nil || s.svc == nil {
		return
	}
	s.svc.Shutdown(context.TODO())
}
