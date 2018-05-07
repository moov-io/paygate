package achsvc

import (
	"context"
	"testing"
)

// TODO add this to setup and tear down testing script
func testService() Service {
	repository := NewInmem()
	repository.Store(&Originator{ID: ResourceID("98765"), Identification: "Not sure about this field", MetaData: "Test Case"})
	return NewService(repository)
}

// TestNewOriginator submits a new originator to store
func TestNewOriginator(t *testing.T) {
	s := testService()
	ctx := context.Background()
	id, err := s.NewOriginator(ctx, Originator{ID: "12345", Identification: "US Bank", MetaData: "Test Case"})
	if id != "12345" {
		t.Errorf("expected %v received %v error %s", "12345", id, err)
	}
}

func TestNewOriginatorIDConflict(t *testing.T) {
	s := testService()
	ctx := context.Background()
	id, err := s.NewOriginator(ctx, Originator{ID: "98765", Identification: "US Bank", MetaData: "Test Case"})
	if err != ErrAlreadyExists {
		t.Errorf("Created Originator when ID 98765 already exists as  %v", id)
	}
}

func TestNewOriginatorNoID(t *testing.T) {
	s := testService()
	ctx := context.Background()
	id, err := s.NewOriginator(ctx, Originator{Identification: "US Bank", MetaData: "Test Case"})
	if err != nil || id == "" {
		t.Errorf("Can't create Originator w/ no ID")

	}

}

func TestOriginatorFound(t *testing.T) {
	s := testService()
	ctx := context.Background()
	o, err := s.Originator(ctx, "98765")
	if o.ID != "98765" {
		t.Errorf("expected %v received %v error %s", "98765", o.ID, err)
	}
}
func TestOriginatorNotFound(t *testing.T) {
	s := testService()
	ctx := context.Background()
	o, err := s.Originator(ctx, "BadID")
	if err != ErrNotFound {
		t.Errorf("expected %v received %v error %s", "98765", o.ID, err)
	}
}

func TestOriginators(t *testing.T) {
	s := testService()
	ctx := context.Background()
	originators := s.Originators(ctx)
	if len(originators) < 1 {
		t.Errorf("Originators less than one result")
	}
}
