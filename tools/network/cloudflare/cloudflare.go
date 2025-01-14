// Copyright (C) 2019 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package cloudflare

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const (
	cloudFlareURI = "https://api.cloudflare.com/client/v4/"
	// AutomaticTTL should be used to request cloudflare's Automatic TTL setting (which is 1).
	AutomaticTTL = 1
)

// DNS is the cloudflare package main access class. Initiate an instance of this class to access the clouldflare APIs.
type DNS struct {
	zoneID    string
	authEmail string
	authKey   string
}

// NewDNS create a new instance of clouldflare DNS services class
func NewDNS(zoneID string, authEmail string, authKey string) *DNS {
	return &DNS{
		zoneID:    zoneID,
		authEmail: authEmail,
		authKey:   authKey,
	}
}

// SetDNSRecord sets the DNS record to the given content.
func (d *DNS) SetDNSRecord(ctx context.Context, recordType string, name string, content string, ttl uint, priority uint, proxied bool) error {
	entries, err := d.ListDNSRecord(ctx, recordType, name, content, "", "", "")
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		fmt.Printf("DNS entry for '%s'='%s' already exists, updating.\n", name, content)
		return d.UpdateDNSRecord(ctx, entries[0].ID, recordType, name, content, ttl, priority, proxied)
	}
	return d.CreateDNSRecord(ctx, recordType, name, content, ttl, priority, proxied)
}

// SetSRVRecord sets the DNS SRV record to the given content.
func (d *DNS) SetSRVRecord(ctx context.Context, name string, target string, ttl uint, priority uint, port uint, service string, protocol string, weight uint) error {
	entries, err := d.ListDNSRecord(ctx, "SRV", service+"."+protocol+"."+name, target, "", "", "")

	if err != nil {
		return err
	}
	if len(entries) != 0 {
		fmt.Printf("SRV entry for '%s'='%s' already exists, updating\n", name, target)
		return d.UpdateSRVRecord(ctx, entries[0].ID, name, target, ttl, priority, port, service, protocol, weight)
	}

	return d.CreateSRVRecord(ctx, name, target, ttl, priority, port, service, protocol, weight)
}

// ClearSRVRecord clears the DNS SRV record to the given content.
func (d *DNS) ClearSRVRecord(ctx context.Context, name string, target string, service string, protocol string) error {
	entries, err := d.ListDNSRecord(ctx, "SRV", service+"."+protocol+"."+name, target, "", "", "")

	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Printf("No SRV entry for '%s'='%s'.\n", name, target)
		return nil
	}

	return d.DeleteDNSRecord(ctx, entries[0].ID)
}

// ListDNSRecord list the dns records that matches the given parameters.
func (d *DNS) ListDNSRecord(ctx context.Context, recordType string, name string, content string, order string, direction string, match string) ([]DNSRecordResponseEntry, error) {
	result := []DNSRecordResponseEntry{}
	const perPage uint = 100
	pageIndex := uint(1)
	queryContent := content
	if recordType == "SRV" {
		queryContent = ""
	}
	for {
		request, err := listDNSRecordRequest(d.zoneID, d.authEmail, d.authKey, recordType, name, queryContent, pageIndex, perPage, order, direction, match)
		if err != nil {
			return []DNSRecordResponseEntry{}, err
		}
		client := &http.Client{}
		response, err := client.Do(request.WithContext(ctx))
		if err != nil {
			return []DNSRecordResponseEntry{}, err
		}

		parsedReponse, err := parseListDNSRecordResponse(response)
		if err != nil {
			return []DNSRecordResponseEntry{}, err
		}
		if len(parsedReponse.Errors) > 0 {
			return []DNSRecordResponseEntry{}, fmt.Errorf("Failed to list DNS entries. %+v", parsedReponse.Errors)
		}
		result = append(result, parsedReponse.Result...)
		if parsedReponse.ResultInfo.TotalPages <= int(pageIndex) {
			break
		}
		pageIndex++
	}
	if recordType == "SRV" && content != "" {
		content = strings.ToLower(content)
		for i := len(result) - 1; i >= 0; i-- {
			if !strings.HasSuffix(strings.ToLower(result[i].Content), content) {
				result = append(result[:i], result[i+1:]...)
			}
		}
	}
	return result, nil
}

// CreateDNSRecord creates the DNS record with the given content.
func (d *DNS) CreateDNSRecord(ctx context.Context, recordType string, name string, content string, ttl uint, priority uint, proxied bool) error {
	request, err := createDNSRecordRequest(d.zoneID, d.authEmail, d.authKey, recordType, name, content, ttl, priority, proxied)
	if err != nil {
		return err
	}
	client := &http.Client{}
	response, err := client.Do(request.WithContext(ctx))
	if err != nil {
		return err
	}

	parsedResponse, err := parseCreateDNSRecordResponse(response)
	if err != nil {
		return err
	}
	if parsedResponse.Success == false {
		return fmt.Errorf("failed to create DNS record : %v", parsedResponse)
	}
	return nil
}

// CreateSRVRecord creates the DNS record with the given content.
func (d *DNS) CreateSRVRecord(ctx context.Context, name string, target string, ttl uint, priority uint, port uint, service string, protocol string, weight uint) error {
	request, err := createSRVRecordRequest(d.zoneID, d.authEmail, d.authKey, name, service, protocol, weight, port, ttl, priority, target)
	if err != nil {
		return err
	}
	client := &http.Client{}
	response, err := client.Do(request.WithContext(ctx))
	if err != nil {
		return err
	}

	parsedResponse, err := parseCreateDNSRecordResponse(response)
	if err != nil {
		return err
	}
	if parsedResponse.Success == false {
		return fmt.Errorf("failed to create SRV record : %v", parsedResponse)
	}
	return nil
}

// DeleteDNSRecord deletes a single DNS entry
func (d *DNS) DeleteDNSRecord(ctx context.Context, recordID string) error {
	request, err := deleteDNSRecordRequest(d.zoneID, d.authEmail, d.authKey, recordID)
	if err != nil {
		return err
	}
	client := &http.Client{}
	response, err := client.Do(request.WithContext(ctx))
	if err != nil {
		return err
	}

	parsedResponse, err := parseDeleteDNSRecordResponse(response)
	if err != nil {
		return err
	}
	if parsedResponse.Success == false {
		return fmt.Errorf("failed to delete DNS record : %v", parsedResponse)
	}
	return nil
}

// UpdateDNSRecord update the DNS record with the given content.
func (d *DNS) UpdateDNSRecord(ctx context.Context, recordID string, recordType string, name string, content string, ttl uint, priority uint, proxied bool) error {
	request, err := updateDNSRecordRequest(d.zoneID, d.authEmail, d.authKey, recordType, recordID, name, content, ttl, priority, proxied)
	if err != nil {
		return err
	}
	client := &http.Client{}
	response, err := client.Do(request.WithContext(ctx))
	if err != nil {
		return err
	}

	parsedResponse, err := parseUpdateDNSRecordResponse(response)
	if err != nil {
		return err
	}
	if parsedResponse.Success == false {
		return fmt.Errorf("failed to update DNS record : %v", parsedResponse)
	}
	return nil
}

// UpdateSRVRecord update the DNS record with the given content.
func (d *DNS) UpdateSRVRecord(ctx context.Context, recordID string, name string, target string, ttl uint, priority uint, port uint, service string, protocol string, weight uint) error {
	request, err := updateSRVRecordRequest(d.zoneID, d.authEmail, d.authKey, recordID, name, service, protocol, weight, port, ttl, priority, target)
	if err != nil {
		return err
	}
	client := &http.Client{}
	response, err := client.Do(request.WithContext(ctx))
	if err != nil {
		return err
	}

	parsedResponse, err := parseUpdateDNSRecordResponse(response)
	if err != nil {
		return err
	}
	if parsedResponse.Success == false {
		return fmt.Errorf("failed to update SRV record : %v", parsedResponse)
	}
	return nil
}
