package achsvc

import (
	"sync"
)

type inmem struct {
	mtx         sync.RWMutex
	originators map[ResourceID]*Originator
}

// NewInmem creates a new in memory repository store
func NewInmem() Repository {
	originator := map[ResourceID]*Originator{}
	return &inmem{
		originators: originator,
	}
}

// PostOriginator creates a new originator resource
func (s *inmem) Store(o *Originator) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if _, ok := s.originators[o.ID]; ok {
		return ErrAlreadyExists
	}
	s.originators[o.ID] = o
	return nil
}

// GetOriginator retrieves the Originator object based on the supplied ID
func (s *inmem) Find(id ResourceID) (*Originator, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	if val, ok := s.originators[id]; ok {
		return val, nil
	}
	return nil, ErrNotFound
}

func (s *inmem) FindAll() []*Originator {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	o := make([]*Originator, 0, len(s.originators))
	for _, val := range s.originators {
		o = append(o, val)
	}
	return o
}
