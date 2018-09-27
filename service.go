// Copyright 2018 The Paygate Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Service is a simple CRUD interface.
type Service interface {
	// NewOriginator creates a new originator
	NewOriginator(ctx context.Context, o Originator) (ResourceID, error)

	Originator(ctx context.Context, id ResourceID) (Originator, error)

	// Originators returns a list of all originators that have been created.
	Originators(ctx context.Context) []Originator
}

// Originator objects are an organization or person that initiates an ACH Transfer to a Customer account either as a debit or credit. The API allows you to create, delete, and update your originators. You can retrieve individual originators as well as a list of all your originators. (Batch Header)
type Originator struct {
	// ID is a globally unique identifier
	ID ResourceID
	// DefaultDepository the depository account to be used by default per transaction.
	DefaultDepository string
	// Identification is a number by which the customer is known to the originator
	Identification string
	// MetaData provides additional data to be used for display and search only
	MetaData string
	// Created a timestamp representing the initial creation date of the object in ISO 8601
	Created string
	// Updated is a timestamp when the object was last modified in ISO8601 format
	Updated string
}

// ResourceID is the ID type of all resource endpoints. Currently implemented as a string but could become UUID, etc.
type ResourceID string

// todo add a String() function

// Repository provides access to a ACH Store
type Repository interface {
	Store(originator *Originator) error
	Find(id ResourceID) (*Originator, error)
	FindAll() []*Originator
}

var (
	ErrNotFound      = errors.New("Not Found")
	ErrAlreadyExists = errors.New("Already Exists")
)

func NewService(r Repository) Service {
	return &service{
		repo: r,
	}
}

// NextID creates a new ID for our system.
// Do no assume anything about these ID's other than
// they are strings. Case matters!
func NextID() ResourceID {
	bs := make([]byte, 20)
	n, err := rand.Read(bs)
	if err != nil || n == 0 {
		logger.Log("generateID", fmt.Sprintf("n=%d, err=%v", n, err))
		return ResourceID("")
	}
	return ResourceID(strings.ToLower(hex.EncodeToString(bs)))
}

type service struct {
	repo Repository
}

// NewOriginator creates a new Originator object
func (s *service) NewOriginator(ctx context.Context, o Originator) (ResourceID, error) {
	if o.ID == "" {
		o.ID = NextID()
	}
	o.Created = time.Now().String()
	o.Updated = time.Now().String()
	/*
		o.Created = fmt.Sprintf("\"%s\"", time.Now().Format("Mon Jan _2"))
		o.Updated = fmt.Sprintf("\"%s\"", time.Now().Format("Mon Jan _2"))
	*/

	if err := s.repo.Store(&o); err != nil {
		return "", err
	}

	return o.ID, nil
}

// Originator retrieves the details of an existing Originator based on the resource id.
func (s *service) Originator(ctx context.Context, id ResourceID) (Originator, error) {
	o, err := s.repo.Find(id)
	if err != nil {
		return Originator{}, ErrNotFound
	}
	return *o, nil
}

// Originators gets a list of Originators
func (s *service) Originators(ctx context.Context) []Originator {
	var result []Originator
	for _, o := range s.repo.FindAll() {
		result = append(result, *o)
	}
	return result
}
