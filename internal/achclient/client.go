// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/antihax/optional"
	"github.com/moov-io/ach"
	moovach "github.com/moov-io/ach/client"
	"github.com/moov-io/base"
	"github.com/moov-io/base/http/bind"
	"github.com/moov-io/base/k8s"
	"github.com/moov-io/paygate"
	"github.com/moov-io/paygate/pkg/id"

	"github.com/go-kit/kit/log"
)

type Client interface {
	GetFile(fileId string) (*ach.File, error)
	GetFileContents(fileId string) (*bytes.Buffer, error)

	CreateFile(idempotencyKey string, req *ach.File) (string, error)
	DeleteFile(fileId string) error

	ValidateFile(fileId string) error

	Ping() error
}

// New creates and returns a Client which can be used to make HTTP requests to an ACH service.
//
// If ran inside a Kubernetes cluster then Moov's kube-dns record will be the default endpoint.
func New(logger log.Logger, endpoint string, userID id.User) Client {
	if endpoint == "" {
		if k8s.Inside() {
			endpoint = "http://ach.apps.svc.cluster.local:8080/"
		} else {
			endpoint = "http://localhost" + bind.HTTP("ach")
		}
	}
	logger.Log("ach", fmt.Sprintf("using %s for ACH address", endpoint))

	u, _ := url.Parse(endpoint)
	if u == nil {
		return nil
	}

	conf := moovach.NewConfiguration()
	conf.Host = u.Host
	conf.BasePath = u.Path
	conf.Scheme = u.Scheme

	conf.AddDefaultHeader("User-Agent", fmt.Sprintf("ach/%s", paygate.Version))
	conf.AddDefaultHeader("X-Idempotency-Key", base.ID())
	conf.AddDefaultHeader("X-User-Id", userID.String())

	return &moovACH{
		logger: logger,
		client: moovach.NewAPIClient(conf),
	}
}

// moovACH is an object for interacting with the Moov ACH service.
//
// This is not intended to be a complete implementation of the API endpoints. Moov offers an OpenAPI specification
// and Go client library that does cover the entire set of API endpoints.
type moovACH struct {
	logger log.Logger
	client *moovach.APIClient
}

// Ping makes an HTTP GET /ping request to the ACH service and returns any errors encountered.
func (a *moovACH) Ping() error {
	if a == nil {
		return errors.New("nil ACH client")
	}
	resp, err := a.client.ACHFilesApi.Ping(context.Background())
	return checkResponse("ping", resp, err)
}

// CreateFile makes HTTP requests to our ACH service in order to create an ACH File.
//
// These Files have many fields associated, but this method performs no validation. However, the
// ACH service might return an error that callers should check.
func (a *moovACH) CreateFile(idempotencyKey string, req *ach.File) (string, error) {
	var request moovach.CreateFile
	request.ID = req.ID
	request.FileHeader = moovach.FileHeader{
		ImmediateOrigin:          req.Header.ImmediateOrigin,
		ImmediateOriginName:      req.Header.ImmediateOriginName,
		ImmediateDestination:     req.Header.ImmediateDestination,
		ImmediateDestinationName: req.Header.ImmediateDestinationName,
		FileCreationTime:         req.Header.FileCreationTime,
		FileCreationDate:         req.Header.FileCreationDate,
		FileIDModifier:           req.Header.FileIDModifier,
	}
	// Batches     []Batch     `json:"batches,omitempty"` // request.Batches = req.Batches.(moovach.Batch)
	// IATBatches  []IatBatch  `json:"IATBatches,omitempty"`
	request.FileControl = moovach.FileControl{
		ID:                req.Control.ID,
		BatchCount:        int32(req.Control.BatchCount),
		BlockCount:        int32(req.Control.BlockCount),
		EntryAddendaCount: int32(req.Control.EntryAddendaCount),
		EntryHash:         int32(req.Control.EntryHash),
		TotalCredit:       int32(req.Control.TotalCreditEntryDollarAmountInFile),
		TotalDebit:        int32(req.Control.TotalDebitEntryDollarAmountInFile),
	}

	fileID, resp, err := a.client.ACHFilesApi.CreateFile(context.Background(), request, &moovach.CreateFileOpts{
		XIdempotencyKey: optional.NewString(idempotencyKey),
	})
	return fileID.ID, checkResponse("create file", resp, err)
}

// ValidateFile makes an HTTP request to our ACH service which performs checks on the
// file to ensure correctness.
func (a *moovACH) ValidateFile(fileId string) error {
	_, resp, err := a.client.ACHFilesApi.ValidateFile(context.Background(), fileId, &moovach.ValidateFileOpts{})
	return checkResponse("validate file", resp, err)
}

// GetFile makes an HTTP request to the ACH service and returns a JSON structure of the ACH file.
func (a *moovACH) GetFile(fileId string) (*ach.File, error) {
	file, resp, err := a.client.ACHFilesApi.GetFileByID(context.Background(), fileId, &moovach.GetFileByIDOpts{})
	return &ach.File{ID: file.ID}, checkResponse("get file", resp, err)
}

// GetFileContents makes an HTTP request to our ACH service and returns the plaintext ACH file.
func (a *moovACH) GetFileContents(fileId string) (*bytes.Buffer, error) {
	str, resp, err := a.client.ACHFilesApi.GetFileContents(context.Background(), fileId, &moovach.GetFileContentsOpts{})
	return bytes.NewBufferString(str), checkResponse("get file contents", resp, err)
}

// DeleteFile makes an HTTP request to our ACH service to delete a file.
func (a *moovACH) DeleteFile(fileId string) error {
	resp, err := a.client.ACHFilesApi.DeleteACHFile(context.Background(), fileId, &moovach.DeleteACHFileOpts{})
	return checkResponse("delete file", resp, err)
}

func checkResponse(label string, resp *http.Response, err error) error {
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("ACH %s %v", label, resp.Status)
	}
	return nil
}
