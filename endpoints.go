package achsvc

import (
	"context"

	"github.com/go-kit/kit/endpoint"
)

type Endpoints struct {
	NewOriginatorEndpoint   endpoint.Endpoint
	LoadOriginatorEndpoint  endpoint.Endpoint
	ListOriginatorsEndpoint endpoint.Endpoint
}

func MakeServerEndpoints(s Service) Endpoints {
	return Endpoints{
		NewOriginatorEndpoint:   MakeNewOriginatorEndpoint(s),
		LoadOriginatorEndpoint:  MakeLoadOriginatorEndpoint(s),
		ListOriginatorsEndpoint: MakeListOriginatorsEndpoint(s),
	}
}

// MakeNewOriginatorEndpoint returns an endpoint via the passed service.
func MakeNewOriginatorEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(newOriginatorRequest)
		id, e := s.NewOriginator(ctx, req.Originator)
		return newOriginatorResponse{ID: id, Err: e}, nil
	}
}

type listOriginatorsRequest struct{}

type listOriginatorsResponse struct {
	Originators []Originator `json:"originators,omitempty"`
	Err         error        `json:"error,omitempty"`
}

func MakeListOriginatorsEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		_ = request.(listOriginatorsRequest)
		return listOriginatorsResponse{Originators: s.Originators(ctx), Err: nil}, nil
	}
}

// MakeLoadOriginatorEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeLoadOriginatorEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(loadOriginatorRequest)
		o, e := s.Originator(ctx, ResourceID(req.ID))
		return loadOriginatorResponse{Originator: o, Err: e}, nil
	}
}

type newOriginatorRequest struct {
	Originator Originator
}

type newOriginatorResponse struct {
	ID  ResourceID `json:"resource_id,omitempty"`
	Err error      `json:"err,omitempty"`
}

func (r newOriginatorResponse) error() error { return r.Err }

type loadOriginatorRequest struct {
	ID string
}

type loadOriginatorResponse struct {
	Originator Originator `json:"originator,omitempty"`
	Err        error      `json:"err,omitempty"`
}

func (r loadOriginatorResponse) error() error { return r.Err }
