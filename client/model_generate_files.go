/*
 * Paygate API
 *
 * Paygate is a RESTful API enabling Automated Clearing House ([ACH](https://en.wikipedia.org/wiki/Automated_Clearing_House)) transactions to be submitted and received without a deep understanding of a full NACHA file specification.
 *
 * API version: v1
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package client

import (
	"time"
)

// GenerateFiles struct for GenerateFiles
type GenerateFiles struct {
	Status    TransferStatus `json:"status,omitempty"`
	StartDate time.Time      `json:"startDate,omitempty"`
	EndDate   time.Time      `json:"endDate,omitempty"`
}
