// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// queryJSON runs a GraphQL query and decodes the response's data into out with
// encoding/json.
//
// Use it for queries that select a Map scalar. The SDK's decoder only decodes
// JSON objects into structs, so a non-empty map fails the whole query there.
//
// GraphQL errors are returned as one error that joins their messages, so
// isNotFoundError works on the result.
func (c *ExtendedGqlClient) queryJSON(ctx context.Context, query string, variables map[string]any, out any) error {
	if c.httpClient == nil {
		return errors.New("the provider's HTTP client is not configured")
	}

	body, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("non-200 OK status code: %v body: %q", resp.Status, msg)
	}

	var payload struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("unable to decode GraphQL response: %w", err)
	}
	if len(payload.Errors) > 0 {
		msgs := make([]string, 0, len(payload.Errors))
		for _, e := range payload.Errors {
			msgs = append(msgs, e.Message)
		}
		return errors.New(strings.Join(msgs, "; "))
	}

	return json.Unmarshal(payload.Data, out)
}
