package achsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/satori/go.uuid"
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

func NextID() ResourceID {
	id, _ := uuid.NewV4()
	//return id.String()
	// make it shorter for testing URL's
	return ResourceID(strings.Split(strings.ToUpper(id.String()), "-")[0])
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
