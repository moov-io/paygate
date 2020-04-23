// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moov-io/base/admin"
	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/util"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

// AddFileTransferConfigRoutes registers the admin HTTP routes for modifying file-transfer (uploading) configs.
func AddFileTransferConfigRoutes(logger log.Logger, svc *admin.Server, repo Repository) {
	svc.AddHandler("/configs/filetransfers", GetConfigs(logger, repo))
	svc.AddHandler("/configs/filetransfers/{routingNumber}", manageFileTransferConfig(logger, repo))
	svc.AddHandler("/configs/filetransfers/cutoff-times/{routingNumber}", manageCutoffTimeConfig(logger, repo))
	svc.AddHandler("/configs/filetransfers/ftp/{routingNumber}", manageFTPConfig(logger, repo))
	svc.AddHandler("/configs/filetransfers/sftp/{routingNumber}", manageSFTPConfig(logger, repo))
}

func getRoutingNumber(r *http.Request) string {
	rtn, ok := mux.Vars(r)["routingNumber"]
	if !ok {
		return ""
	}
	return rtn
}

type adminConfigResponse struct {
	CutoffTimes         []*CutoffTime `json:"CutoffTimes"`
	FileTransferConfigs []*Config     `json:"Configs"`
	FTPConfigs          []*FTPConfig  `json:"FTPConfigs"`
	SFTPConfigs         []*SFTPConfig `json:"SFTPConfigs"`
}

// GetConfigs returns all configurations (i.e. FTP, cutoff times, file-transfer configs with passwords masked. (e.g. 'p******d')
func GetConfigs(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}

		resp := &adminConfigResponse{}
		if v, err := repo.GetCutoffTimes(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.CutoffTimes = v
		}
		if v, err := repo.GetConfigs(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.FileTransferConfigs = v
		}
		if v, err := repo.GetFTPConfigs(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.FTPConfigs = maskFTPPasswords(v)
		}
		if v, err := repo.GetSFTPConfigs(); err != nil {
			moovhttp.Problem(w, err)
			return
		} else {
			resp.SFTPConfigs = maskSFTPPasswords(v)
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func maskPassword(s string) string {
	if utf8.RuneCountInString(s) < 3 {
		return "**" // too short, we can't mask anything
	} else {
		// turn 'password' into 'p******d'
		first, last := s[0:1], s[len(s)-1:]
		return fmt.Sprintf("%s%s%s", first, strings.Repeat("*", len(s)-2), last)
	}
}

func maskFTPPasswords(cfgs []*FTPConfig) []*FTPConfig {
	for i := range cfgs {
		cfgs[i].Password = maskPassword(cfgs[i].Password)
	}
	return cfgs
}

func maskSFTPPasswords(cfgs []*SFTPConfig) []*SFTPConfig {
	for i := range cfgs {
		cfgs[i].Password = maskPassword(cfgs[i].Password)
	}
	return cfgs
}

func manageFileTransferConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routingNumber := getRoutingNumber(r)
		if routingNumber == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case "PUT":
			type request struct {
				InboundPath              string `json:"inboundPath,omitempty"`
				OutboundPath             string `json:"outboundPath,omitempty"`
				ReturnPath               string `json:"returnPath,omitempty"`
				OutboundFilenameTemplate string `json:"outboundFilenameTemplate,omitempty"`
				AllowedIPs               string `json:"allowedIPs,omitempty"`
			}
			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			// Ensure that a provided template validates before saving it
			if req.OutboundFilenameTemplate != "" {
				if err := validateTemplate(req.OutboundFilenameTemplate); err != nil {
					moovhttp.Problem(w, err)
					return
				}
			}
			existing := readFileTransferConfig(repo, routingNumber)
			err := repo.upsertConfig(&Config{
				RoutingNumber:            routingNumber,
				InboundPath:              util.Or(req.InboundPath, existing.InboundPath),
				OutboundPath:             util.Or(req.OutboundPath, existing.OutboundPath),
				ReturnPath:               util.Or(req.ReturnPath, existing.ReturnPath),
				OutboundFilenameTemplate: util.Or(req.OutboundFilenameTemplate, existing.OutboundFilenameTemplate),
				AllowedIPs:               util.Or(req.AllowedIPs, existing.AllowedIPs),
			})
			if err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("updated config for routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))
			w.WriteHeader(http.StatusOK)

		case "DELETE":
			if err := repo.deleteConfig(routingNumber); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("deleted config for routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))
			w.WriteHeader(http.StatusOK)

		default:
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
	}
}

func manageCutoffTimeConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routingNumber := getRoutingNumber(r)
		if routingNumber == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case "PUT":
			type request struct {
				Cutoff   int    `json:"cutoff"`
				Location string `json:"location"`
			}
			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			if req.Cutoff == 0 {
				moovhttp.Problem(w, errors.New("misisng cutoff"))
				return
			}
			loc, err := time.LoadLocation(req.Location)
			if err != nil {
				moovhttp.Problem(w, fmt.Errorf("time: %s: %v", req.Location, err))
				return
			}
			if err := repo.upsertCutoffTime(routingNumber, req.Cutoff, loc); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("updating cutoff time config routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		case "DELETE":
			if err := repo.deleteCutoffTime(routingNumber); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("deleting cutoff time config routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		default:
			moovhttp.Problem(w, fmt.Errorf("cutoff-times: unsupported HTTP verb %s", r.Method))
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func manageFTPConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routingNumber := getRoutingNumber(r)
		if routingNumber == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case "PUT":
			type request struct {
				Hostname string `json:"hostname"`
				Username string `json:"username"`
				Password string `json:"password,omitempty"`
			}
			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			if req.Hostname == "" || req.Username == "" {
				moovhttp.Problem(w, errors.New("missing hostname, or username"))
				return
			}
			if err := repo.upsertFTPConfigs(routingNumber, req.Hostname, req.Username, req.Password); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("updating FTP configs routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		case "DELETE":
			if err := repo.deleteFTPConfig(routingNumber); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("deleting FTP config routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		default:
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func manageSFTPConfig(logger log.Logger, repo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routingNumber := getRoutingNumber(r)
		if routingNumber == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case "PUT":
			type request struct {
				Hostname         string `json:"hostname"`
				Username         string `json:"username"`
				Password         string `json:"password,omitempty"`
				ClientPrivateKey string `json:"clientPrivateKey,omitempty"`
				HostPublicKey    string `json:"hostPublicKey,omitempty"`
			}
			var req request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			if req.Hostname == "" || req.Username == "" {
				moovhttp.Problem(w, errors.New("missing hostname, or username"))
				return
			}
			if err := repo.upsertSFTPConfigs(routingNumber, req.Hostname, req.Username, req.Password, req.ClientPrivateKey, req.HostPublicKey); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("updating SFTP config routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		case "DELETE":
			if err := repo.deleteSFTPConfig(routingNumber); err != nil {
				moovhttp.Problem(w, err)
				return
			}
			logger.Log("file-transfer-configs", fmt.Sprintf("deleting SFTP cofnig routingNumber=%s", routingNumber), "requestID", moovhttp.GetRequestID(r))

		default:
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb %s", r.Method))
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
