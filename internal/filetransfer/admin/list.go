// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/moov-io/ach"
	moovhttp "github.com/moov-io/base/http"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type mergedFiles struct {
	Files []mergedFile `json:"files"`
}

type mergedFile struct {
	Filename string      `json:"filename"`
	Header   *fileHeader `json:"header"`
}

type fileHeader struct {
	Origin          string `json:"origin"`
	OriginName      string `json:"originName"`
	Destination     string `json:"destination"`
	DestinationName string `json:"destinationName"`
}

func listMergedFiles(logger log.Logger, getFiles func() ([]string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := getFiles()
		if err != nil {
			fmt.Printf("error=%v\n", err)
		}

		var out []mergedFile
		for i := range files {
			fd, err := os.Open(files[i])
			if err != nil {
				continue // skip, might have been renamed from under us
			}
			fh, err := readFileHeader(fd)
			fd.Close()
			if err != nil {
				continue // skip
			}
			out = append(out, mergedFile{
				Filename: filepath.Base(fd.Name()),
				Header:   fh,
			})
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&mergedFiles{Files: out})
	}
}

func getRouteFilename(r *http.Request) string {
	v, _ := mux.Vars(r)["filename"]
	return v
}

func getMergedFile(logger log.Logger, getFiles func() ([]string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := getFiles()
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}
		filename := filepath.Base(getRouteFilename(r))

		for i := range files {
			if strings.EqualFold(filepath.Base(files[i]), filename) {
				file, err := readACHFilepath(files[i])
				if err != nil {
					moovhttp.Problem(w, err)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(file)
				return
			}
		}

		moovhttp.Problem(w, fmt.Errorf("file %s not found", filename))
	}
}

// readFileHeader hooks into knowledge of an ACH file to read the just the FileHeader
// information instead of the entire file's contents.
func readFileHeader(fd *os.File) (*fileHeader, error) {
	line := make([]byte, ach.RecordLength)
	if n, err := fd.Read(line); n != ach.RecordLength || err != nil {
		return nil, fmt.Errorf("read %d bytes: %v", n, err)
	}

	fh := ach.NewFileHeader()
	fh.Parse(string(line))

	return &fileHeader{
		Origin:          fh.ImmediateOrigin,
		OriginName:      fh.ImmediateOriginName,
		Destination:     fh.ImmediateDestination,
		DestinationName: fh.ImmediateDestinationName,
	}, nil
}

func readACHFilepath(path string) (*ach.File, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	file, err := ach.NewReader(fd).Read()
	if err != nil {
		return nil, err
	}
	return &file, nil
}
