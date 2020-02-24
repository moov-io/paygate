// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package depository

import (
	"encoding/json"
	"fmt"
	"net/http"

	moovhttp "github.com/moov-io/base/http"
	"github.com/moov-io/paygate/internal/model"
	"github.com/moov-io/paygate/internal/route"

	"github.com/go-kit/kit/log"
)

type request struct {
	Status model.DepositoryStatus `json:"status"`
}

func overrideDepositoryStatus(logger log.Logger, depRepo Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w = route.Wrap(logger, w, r)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if r.Method != "PUT" {
			moovhttp.Problem(w, fmt.Errorf("unsupported HTTP verb: %s", r.Method))
			return
		}

		depID := GetDepositoryID(r)
		requestID, userID := moovhttp.GetRequestID(r), route.GetUserID(r)

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			moovhttp.Problem(w, err)
			return
		}

		// read the depository so we know it exists
		dep, err := depRepo.GetDepository(depID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}
		if err := depRepo.UpdateDepositoryStatus(depID, req.Status); err != nil {
			moovhttp.Problem(w, err)
			return
		}
		// re-read for marshaling
		dep, err = depRepo.GetUserDepository(depID, dep.UserID)
		if err != nil {
			moovhttp.Problem(w, err)
			return
		}

		logger.Log(
			"depositories", fmt.Sprintf("updated depository=%s to %s", depID, req.Status),
			"requestID", requestID, "userID", userID)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&dep)
	}
}
