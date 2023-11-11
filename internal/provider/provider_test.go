// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

type serviceAccountCredentials struct {
	Mrn         string `json:"mrn,omitempty"`
	ParentMrn   string `json:"parent_mrn,omitempty"`
	PrivateKey  string `json:"private_key,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	ApiEndpoint string `json:"api_endpoint,omitempty"`
}

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"mondoo": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	// nothing to do here for now
}

func getOrgId() (string, error) {
	// You can add code here to run prior to any test case execution, for example assertions
	// about the appropriate environment variables being set are common to see in a pre-check
	// function.
	var testCredentials serviceAccountCredentials

	path := os.Getenv("MONDOO_CONFIG_PATH")
	base64value := os.Getenv("MONDOO_CONFIG_BASE64")

	if base64value != "" {
		data, err := base64.StdEncoding.DecodeString(base64value)
		if err != nil {
			return "", errors.New("MONDOO_CONFIG_BASE64 must be a valid service account")
		}
		err = json.Unmarshal(data, &testCredentials)
		if err != nil {
			return "", errors.New("MONDOO_CONFIG_BASE64 must be a valid service account")
		}
	} else if path != "" {
		file, err := os.Open(path)
		if err != nil {
			return "", errors.New("MONDOO_CONFIG_PATH must be a valid service account")
		}

		err = json.NewDecoder(file).Decode(&testCredentials)
		if err != nil {
			return "", errors.New("MONDOO_CONFIG_PATH must be a valid service account")
		}
	}

	// extract orgID from service account mrn
	orgIdRegexp := regexp.MustCompile(`\/\/agents.api.mondoo.app\/organizations\/([\d\w-]+)\/`)

	m := orgIdRegexp.FindStringSubmatch(testCredentials.Mrn)
	if len(m) == 2 {
		return m[1], nil
	} else {
		return "", errors.New("MONDOO_CONFIG_PATH or MONDOO_CONFIG_BASE64 must be a valid organization service account")
	}
}
