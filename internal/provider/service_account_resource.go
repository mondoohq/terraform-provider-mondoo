// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ServiceAccountResource{}

//var _ resource.ResourceWithImportState = &ServiceAccountResource{}

var defaultRoles = []string{"//iam.api.mondoo.app/roles/viewer"}

// serviceAccountCredential is a temporary object until the API returns the credential as a string.
type serviceAccountCredential struct {
	Mrn         string `json:"mrn,omitempty"`
	PrivateKey  string `json:"private_key,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	ApiEndpoint string `json:"api_endpoint,omitempty"`
	ScopeMrn    string `json:"scope_mrn,omitempty"`
	// ParentMrn is deprecated and should not be used, use ScopeMrn instead
	ParentMrn string `json:"parent_mrn,omitempty"`
}

func NewServiceAccountResource() resource.Resource {
	return &ServiceAccountResource{}
}

// ServiceAccountResource defines the resource implementation.
type ServiceAccountResource struct {
	client *ExtendedGqlClient
}

// ServiceAccountResourceModel describes the resource data model.
type ServiceAccountResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`
	OrgID   types.String `tfsdk:"org_id"`

	// service account details
	Mrn         types.String `tfsdk:"mrn"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Roles       types.List   `tfsdk:"roles"`

	// Base64 encoded service account credential
	Credential types.String `tfsdk:"credential"`
}

func (r *ServiceAccountResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *ServiceAccountResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: `Allows management of a Mondoo service account.`,

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the service account.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the service account.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Created by Terraform"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mrn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Mondoo resource name (MRN) of the created service account.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"space_id": schema.StringAttribute{ // TODO: add check that either space or org needs to be set
				MarkdownDescription: "The identifier of the Mondoo space in which to create the service account.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"org_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the Mondoo organization in which to create the service account.",
				Optional:            true,
			},
			"roles": schema.ListAttribute{
				MarkdownDescription: "Roles to assign to the service account.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"credential": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The service account credential in JSON format, base64 encoded. This is the same content when creating service account credentials through the Mondoo Console.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Sensitive: true,
			},
		},
	}
}

func (r *ServiceAccountResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *ServiceAccountResource) getScope(ctx context.Context, data ServiceAccountResourceModel) string {
	// default to platform level
	scopeMrn := "//platform.api.mondoo.app"
	// Give presedence to the org id
	if data.OrgID.ValueString() != "" {
		scopeMrn = orgPrefix + data.OrgID.ValueString()
		ctx = tflog.SetField(ctx, "org_mrn", scopeMrn)
	} else if space, err := r.client.ComputeSpace(data.SpaceID); err == nil {
		scopeMrn = space.MRN()
		ctx = tflog.SetField(ctx, "space_mrn", scopeMrn)
	}
	tflog.Debug(ctx, "Scope for service account")
	return scopeMrn
}

func (r *ServiceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceAccountResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource
	name := data.Name.ValueString()

	roles := []string{}
	if len(data.Roles.Elements()) == 0 {
		var d diag.Diagnostics
		data.Roles, d = types.ListValueFrom(ctx, types.StringType, defaultRoles)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	d := data.Roles.ElementsAs(ctx, &roles, false)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	rolesInput := []mondoov1.RoleInput{}
	for _, role := range roles {
		rolesInput = append(rolesInput, mondoov1.RoleInput{Mrn: mondoov1.String(role)})
	}

	scopeMrn := r.getScope(ctx, data)
	if scopeMrn == "" {
		resp.Diagnostics.AddError(
			"Either space_id or org_id needs to be set",
			"Either space_id or org_id needs to be set",
		)
		return
	}

	createInput := mondoov1.CreateServiceAccountInput{
		Name:        mondoov1.NewStringPtr(mondoov1.String(name)),
		Description: mondoov1.NewStringPtr(mondoov1.String(data.Description.ValueString())),
		ScopeMrn:    mondoov1.String(scopeMrn),
		Roles:       &rolesInput,
	}

	tflog.Debug(ctx, "CreateSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createInput),
	})

	var createMutation struct {
		CreateServiceAccount struct {
			Mrn         mondoov1.String
			Certificate mondoov1.String
			PrivateKey  mondoov1.String
			ScopeMrn    mondoov1.String
			ApiEndpoint mondoov1.String
		} `graphql:"createServiceAccount(input: $input)"`
	}

	err := r.client.Mutate(ctx, &createMutation, createInput, nil)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create service account. Got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Name = types.StringValue(name)
	data.Mrn = types.StringValue(string(createMutation.CreateServiceAccount.Mrn))

	// NOTE: this is temporary, we want to change the API to return the credential as a string
	serviceAccount := serviceAccountCredential{
		Mrn:         string(createMutation.CreateServiceAccount.Mrn),
		PrivateKey:  string(createMutation.CreateServiceAccount.PrivateKey),
		Certificate: string(createMutation.CreateServiceAccount.Certificate),
		ApiEndpoint: string(createMutation.CreateServiceAccount.ApiEndpoint),
		ScopeMrn:    string(createMutation.CreateServiceAccount.ScopeMrn),
		ParentMrn:   string(createMutation.CreateServiceAccount.ScopeMrn),
	}

	jsonData, err := json.Marshal(serviceAccount)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create service account. Got error: %s", err),
			)
		return
	}

	// set Base64 encoded credential
	data.Credential = types.StringValue(base64.StdEncoding.EncodeToString(jsonData))

	// Write logs using the tflog package
	tflog.Debug(ctx, "created a service account resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountResource) readServiceAccount(ctx context.Context, mrn string) (ServiceAccountResourceModel, error) {
	var q struct {
		ServiceAccount struct {
			Id          string
			Mrn         string
			Name        string
			Description string
			Roles       []struct {
				Mrn string
			}
			Labels []struct {
				Key   string
				Value string
			}
		} `graphql:"serviceAccount(mrn: $mrn)"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}

	err := r.client.Query(ctx, &q, variables)
	if err != nil {
		return ServiceAccountResourceModel{}, err
	}

	return ServiceAccountResourceModel{
		Mrn:         types.StringValue(q.ServiceAccount.Mrn),
		Name:        types.StringValue(q.ServiceAccount.Name),
		Description: types.StringValue(q.ServiceAccount.Description),
		// TODO: add roles
		//SpaceID: types.StringValue(q.ServiceAccount.Id),
		//OrgID:   types.StringValue(q.Space.Organization.Id),
	}, nil
}

func (r *ServiceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceAccountResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	m, err := r.readServiceAccount(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to read service account. Got error: %s", err),
		)
		return
	}

	data.Mrn = m.Mrn
	data.Name = m.Name
	data.Description = m.Description

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServiceAccountResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	var updateMutation struct {
		UpdateServiceAccount struct {
			Mrn         mondoov1.String
			Name        mondoov1.String
			Description mondoov1.String
		} `graphql:"updateServiceAccount(input: $input)"`
	}
	updateInput := mondoov1.UpdateServiceAccountInput{
		Mrn:   mondoov1.String(data.Mrn.ValueString()),
		Name:  mondoov1.NewStringPtr(mondoov1.String(data.Name.ValueString())),
		Notes: mondoov1.NewStringPtr(mondoov1.String(data.Description.ValueString())),
	}
	tflog.Debug(ctx, "UpdateServiceAccountInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", updateInput),
	})
	err := r.client.Mutate(ctx, &updateMutation, updateInput, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update service account. Got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.State.SetAttribute(ctx, path.Root("name"), updateMutation.UpdateServiceAccount.Name)
}

func (r *ServiceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceAccountResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	scopeMrn := r.getScope(ctx, data)
	if scopeMrn == "" {
		resp.Diagnostics.AddError(
			"Either space_id or org_id must be set",
			"Either space_id or org_id must be set",
		)
		return
	}

	// Do GraphQL request to API to delete the resource.
	var deleteMutation struct {
		DeleteServiceAccounts struct {
			Mrns []mondoov1.String
		} `graphql:"deleteServiceAccounts(input: $input)"`
	}
	deleteInput := mondoov1.DeleteServiceAccountsInput{
		ScopeMrn: mondoov1.String(scopeMrn),
		Mrns:     []mondoov1.String{mondoov1.String(data.Mrn.ValueString())},
	}
	tflog.Debug(ctx, "UpdateServiceAccountInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", deleteInput),
	})
	err := r.client.Mutate(ctx, &deleteMutation, deleteInput, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update service account. Got error: %s", err))
		return
	}
}

// We do not allow users to import a service account resource.
// func (r *ServiceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {}
