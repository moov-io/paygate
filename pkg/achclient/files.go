// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package achclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/moov-io/ach"
)

type createFileResponse struct {
	ID    string `json:"id"`
	Error error  `json:"error,omitempty"`
}

// CreateFile makes HTTP requests to our ACH service in order to create an ACH File.
//
// These Files have many fields associated, but this method performs no validation. However, the
// ACH service might return an error that callers should check.
func (a *ACH) CreateFile(idempotencyKey string, req *ach.File) (string, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&req); err != nil || buf.Len() == 0 {
		return "", fmt.Errorf("CreateFile: file ID %s json encoding error: %v", req.ID, err)
	}
	resp, err := a.POST("/files/create", idempotencyKey, ioutil.NopCloser(&buf))
	if err != nil {
		return "", fmt.Errorf("CreateFile: error file ID %s : %v", req.ID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("CreateFile: file ID %s got %d HTTP status", req.ID, resp.StatusCode)
	}

	// Read response
	var response createFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("CreateFile: problem reading response: %v", err)
	}
	return response.ID, response.Error
}

type validateFileResponse struct {
	Err string `json:"error"`
}

// ValidateFile makes an HTTP request to our ACH service which performs checks on the
// file to ensure correctness.
func (a *ACH) ValidateFile(fileId string) error {
	resp, err := a.GET(fmt.Sprintf("/files/%s/validate", fileId))
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	// Try reading error from ACH service
	var response validateFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("ValidateFile: problem reading response json: %v", err)
	}
	if response.Err != "" {
		return fmt.Errorf("ValidateFile (fileId=%s): %s", fileId, response.Err)
	}

	// Just return the a.GET error
	if err != nil {
		return fmt.Errorf("ValidateFile: error making HTTP request: %v", err)
	}
	return nil
}

func (a *ACH) GetFile(fileId string) (*ach.File, error) {
	resp, err := a.GET(fmt.Sprintf("/files/%s", fileId))
	if err != nil {
		return nil, fmt.Errorf("GetFile: error making HTTP request: %v", err)
	}
	defer resp.Body.Close()

	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GetFile: problem reading ACH response: %v", err)
	}

	file, err := ach.FileFromJSON(bs)
	if err != nil {
		return nil, fmt.Errorf("GetFile: error parsing ach.File: %v", err)
	}
	return file, nil
}

// GetFileContents makes an HTTP request to our ACH service and returns the plaintext ACH file.
func (a *ACH) GetFileContents(fileId string) (*bytes.Buffer, error) {
	resp, err := a.GET(fmt.Sprintf("/files/%s/contents", fileId))
	if err != nil {
		return nil, fmt.Errorf("GetFileContents: error making HTTP request: %v", err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	n, err := io.Copy(&buf, resp.Body)
	if err != nil || n == 0 {
		return nil, fmt.Errorf("GetFileContents: problem reading body (n=%d): %v", n, err)
	}
	return &buf, nil
}

// DeleteFile makes an HTTP request to our ACH service to delete a file.
func (a *ACH) DeleteFile(fileId string) error {
	resp, err := a.DELETE(fmt.Sprintf("/files/%s", fileId))
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil // ach.File not found
	}
	if err != nil {
		return fmt.Errorf("DeleteFile: problem with HTTP request: %v", err)
	}
	resp.Body.Close()
	return nil
}
