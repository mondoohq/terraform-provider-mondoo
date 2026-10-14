// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBigQueryKmsConfiguration(t *testing.T) {
	const key = "projects/customer/locations/europe/keyRings/exports/cryptoKeys/bigquery"
	for _, configured := range []types.String{types.StringNull(), types.StringValue(key)} {
		for _, wif := range []bool{false, true} {
			model := BigQueryExportResourceModel{DatasetID: types.StringValue("project.dataset"), KmsKeyName: configured, ServiceAccountKey: types.StringNull()}
			if wif {
				model.Credentials = &exportBigQueryCredentialsWrapper{Wif: &gcpWifCredentialModel{Audience: types.StringValue("audience"), ServiceAccountEmail: types.StringNull()}}
			} else {
				model.ServiceAccountKey = types.StringValue("service-account-json")
			}
			opts := model.GetConfigurationOptions()
			require.NotNil(t, opts.KmsKeyName)
			assert.Equal(t, configured.ValueString(), string(*opts.KmsKeyName))
			data, err := json.Marshal(opts)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(data, &payload))
			assert.Equal(t, configured.ValueString(), payload["kmsKeyName"], "removal must send an empty string, not omit the field")
		}
	}
}

func TestBigQueryKmsSchema(t *testing.T) {
	var response resource.SchemaResponse
	NewMondooExportBigQueryResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	attribute, ok := response.Schema.Attributes["kms_key_name"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, attribute.Optional)
	assert.False(t, attribute.Sensitive, "the resource name is not key material")
	assert.Empty(t, attribute.PlanModifiers, "changing a key must not replace the integration")
}
