// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*WorkspaceResource)(nil)

func NewWorkspaceResource() resource.Resource {
	return &WorkspaceResource{}
}

// WorkspaceResource defines the resource implementation.
type WorkspaceResource struct {
	client *ExtendedGqlClient
}

// WorkspaceResourceModel describes the resource data model.
type WorkspaceResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// Workspace details
	//
	// Mondoo resource name
	Mrn types.String `tfsdk:"mrn"`
	// User selected name. (Required.)
	Name types.String `tfsdk:"name"`
	// Optional description. (Optional.)
	Description types.String `tfsdk:"description"`
	// A list of workspace selections. (Required.)
	Selections []WorkspaceSelectionModel `tfsdk:"asset_selections"`
}

type WorkspaceSelectionModel struct {
	// A list of conditions for the selection. (Required.)
	Conditions []WorkspaceConditionModel `tfsdk:"conditions"`
}

type WorkspaceConditionModel struct {
	// Operator determining how the condition is joined with the other conditions in the list. (Required.)
	Operator types.String `tfsdk:"operator"`

	// String condition. (Optional.)
	StringCondition *WorkspaceGenericCondition `tfsdk:"string_condition"`
	// Int condition. (Optional.)
	IntCondition *WorkspaceGenericCondition `tfsdk:"int_condition"`
	// Rating condition. (Optional.)
	RatingCondition *WorkspaceGenericCondition `tfsdk:"rating_condition"`
	// Key-value condition. (Optional.)
	KeyValueCondition *WorkspaceKeyValueCondition `tfsdk:"key_value_condition"`
}

type WorkspaceGenericCondition struct {
	// Field to match. (Required.)
	Field types.String `tfsdk:"field"`
	// Operator to use. (Required.)
	Operator types.String `tfsdk:"operator"`
	// Values to match. Values are ORed together. (Required.)
	Values types.List `tfsdk:"values"`
}

type WorkspaceKeyValueCondition struct {
	// Field to match. (Required.)
	Field types.String `tfsdk:"field"`
	// Operator to use. (Required.)
	Operator types.String `tfsdk:"operator"`
	// Values to match. Values are ORed together. (Required.)
	Values []WorkspaceKeyValue `tfsdk:"values"`
}
type WorkspaceKeyValue struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

func (r *WorkspaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (r *WorkspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: `Allows management of Mondoo workspaces.`,

		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo space identifier. If there is no ID, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mrn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Mondoo resource name (MRN) of the workspace.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the workspace.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the workspace.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Created by Terraform"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"asset_selections": schema.ListNestedAttribute{
				MarkdownDescription: "A list of workspace selections.",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"conditions": schema.ListNestedAttribute{
							Required:            true,
							MarkdownDescription: "A list of conditions for the selection.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"operator": schema.StringAttribute{
										Required: true,
										MarkdownDescription: "Operator determining how the condition is joined with the other " +
											"conditions in the list. Valid values: `AND`, `AND_NOT`",
									},
									"string_condition": schema.SingleNestedAttribute{
										MarkdownDescription: "A condition with values of type string.",
										Optional:            true,
										Attributes: map[string]schema.Attribute{
											"field": schema.StringAttribute{
												MarkdownDescription: fmt.Sprintf(
													"String field to match. Valid values: %q", displayPossibleStringFields(),
												),
												Required: true,
											},
											"operator": schema.StringAttribute{
												MarkdownDescription: fmt.Sprintf(
													"String operator. Valid values: %q", displayPossibleStringOperators(),
												),
												Required: true,
											},
											"values": schema.ListAttribute{
												MarkdownDescription: "String values to match. Values are ORed together.",
												ElementType:         types.StringType,
												Required:            true,
											},
										},
									},
									"int_condition": schema.SingleNestedAttribute{
										MarkdownDescription: "A condition with values of type int.",
										Optional:            true,
										Attributes: map[string]schema.Attribute{
											"field": schema.StringAttribute{
												MarkdownDescription: fmt.Sprintf(
													"Numeric field to match. Valid values: %q", displayPossibleIntFields(),
												),
												Required: true,
											},
											"operator": schema.StringAttribute{
												MarkdownDescription: fmt.Sprintf(
													"Numeric operator. Valid values: %q", displayPossibleNumericOperators(),
												),
												Required: true,
											},
											"values": schema.ListAttribute{
												MarkdownDescription: "Int values to match. Values are ORed together.",
												ElementType:         types.Int32Type,
												Required:            true,
											},
										},
									},
									"rating_condition": schema.SingleNestedAttribute{
										MarkdownDescription: "A condition with values of type int.",
										Optional:            true,
										Attributes: map[string]schema.Attribute{
											"field": schema.StringAttribute{
												MarkdownDescription: fmt.Sprintf(
													"Rating field to match. Valid values: %q", displayPossibleRatingFields(),
												),
												Required: true,
											},
											"operator": schema.StringAttribute{
												MarkdownDescription: fmt.Sprintf(
													"Rating operator. Valid values: %q", displayPossibleRatingOperators(),
												),
												Required: true,
											},
											"values": schema.ListAttribute{
												MarkdownDescription: "Int values to match. Values are ORed together.",
												ElementType:         types.StringType,
												Required:            true,
											},
										},
									},
									"key_value_condition": schema.SingleNestedAttribute{
										MarkdownDescription: "A condition with values of type key:value.",
										Optional:            true,
										Attributes: map[string]schema.Attribute{
											"field": schema.StringAttribute{
												MarkdownDescription: fmt.Sprintf(
													"key:value field to match. Valid values: %q",
													displayPossibleKeyValueFields(),
												),
												Required: true,
											},
											"operator": schema.StringAttribute{
												MarkdownDescription: fmt.Sprintf(
													"Rating operator. Valid values: %q", displayPossibleKeyValueOperators(),
												),
												Required: true,
											},
											"values": schema.ListNestedAttribute{
												MarkdownDescription: "key:value list to match. Values are ORed together.",
												Required:            true,
												NestedObject: schema.NestedAttributeObject{
													Attributes: map[string]schema.Attribute{
														"key": schema.StringAttribute{
															MarkdownDescription: "The key.",
															Required:            true,
														},
														"value": schema.StringAttribute{
															MarkdownDescription: "The value.",
															Required:            true,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *WorkspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func renderSelectionsFromGraphql(response WorkspaceSelections) []WorkspaceSelectionModel {
	selectionModel := make([]WorkspaceSelectionModel, len(response.Selections))
	for i, selection := range response.Selections {
		conditionsModel := make([]WorkspaceConditionModel, len(selection.Conditions))
		for j, condition := range selection.Conditions {
			newCondition := WorkspaceConditionModel{
				Operator: types.StringValue(string(condition.Operator)),
			}
			switch condition.Condition.Typename {
			case "WorkspaceSelectionKeyValueCondition":
				newCondition.KeyValueCondition = &WorkspaceKeyValueCondition{
					Field:    types.StringValue(string(condition.Condition.KeyValueCondition.Field)),
					Operator: types.StringValue(string(condition.Condition.KeyValueCondition.Operator)),
				}
				for _, kv := range condition.Condition.KeyValueCondition.Values {
					newCondition.KeyValueCondition.Values = append(newCondition.KeyValueCondition.Values,
						WorkspaceKeyValue{
							Key:   types.StringValue(kv.Key),
							Value: types.StringValue(kv.Value),
						})
				}
			case "WorkspaceSelectionStringCondition":
				newCondition.StringCondition = &WorkspaceGenericCondition{
					Field:    types.StringValue(string(condition.Condition.StringCondition.Field)),
					Operator: types.StringValue(string(condition.Condition.StringCondition.Operator)),
					Values:   ConvertListValue(condition.Condition.StringCondition.ValuesStringSlice()),
				}
			case "WorkspaceSelectionIntCondition":
				newCondition.IntCondition = &WorkspaceGenericCondition{
					Field:    types.StringValue(string(condition.Condition.IntCondition.Field)),
					Operator: types.StringValue(string(condition.Condition.IntCondition.Operator)),
					Values:   ConvertListValueInt32(condition.Condition.IntCondition.Values),
				}
			case "WorkspaceSelectionRatingCondition":
				newCondition.RatingCondition = &WorkspaceGenericCondition{
					Field:    types.StringValue(string(condition.Condition.RatingCondition.Field)),
					Operator: types.StringValue(string(condition.Condition.RatingCondition.Operator)),
					Values:   ConvertListValue(condition.Condition.RatingCondition.Values),
				}
			}

			conditionsModel[j] = newCondition
		}
		selectionModel[i].Conditions = conditionsModel
	}
	return selectionModel
}

func renderSelectionsFromModel(data *WorkspaceResourceModel) mondoov1.WorkspaceSelectionsInput {
	selectionInput := mondoov1.WorkspaceSelectionsInput{
		Selections: []mondoov1.WorkspaceSelectionInput{},
	}
	for _, selection := range data.Selections {
		newSelection := mondoov1.WorkspaceSelectionInput{
			Conditions: []mondoov1.WorkspaceSelectionConditionInput{},
		}
		for _, condition := range selection.Conditions {

			newCondition := mondoov1.WorkspaceSelectionConditionInput{
				Operator: mondoov1.WorkspaceSelectionConditionOperator(condition.Operator.ValueString()),
			}
			if condition.KeyValueCondition != nil {
				var values []mondoov1.KeyValueInput
				for _, kv := range condition.KeyValueCondition.Values {
					values = append(values, mondoov1.KeyValueInput{
						Key:   mondoov1.String(kv.Key.ValueString()),
						Value: mondoov1.NewStringPtr(mondoov1.String(kv.Value.ValueString())),
					})
				}
				newCondition.KeyValueCondition = &mondoov1.WorkspaceSelectionKeyValueConditionInput{
					Field:    mondoov1.WorkspaceSelectionConditionKeyValueField(condition.KeyValueCondition.Field.ValueString()),
					Operator: mondoov1.WorkspaceSelectionConditionKeyValueOperator(condition.KeyValueCondition.Operator.ValueString()),
					Values:   values,
				}
			}
			if condition.IntCondition != nil {
				newCondition.IntCondition = &mondoov1.WorkspaceSelectionIntConditionInput{
					Field:    mondoov1.WorkspaceSelectionConditionIntField(condition.IntCondition.Field.ValueString()),
					Operator: mondoov1.WorkspaceSelectionConditionNumericOperator(condition.IntCondition.Operator.ValueString()),
					Values:   ConvertSlice[mondoov1.Int](condition.IntCondition.Values),
				}
			}
			if condition.StringCondition != nil {
				newCondition.StringCondition = &mondoov1.WorkspaceSelectionStringConditionInput{
					Field:    mondoov1.WorkspaceSelectionConditionStringField(condition.StringCondition.Field.ValueString()),
					Operator: mondoov1.WorkspaceSelectionConditionStringOperator(condition.StringCondition.Operator.ValueString()),
					Values:   ConvertSlice[mondoov1.String](condition.StringCondition.Values),
				}
			}
			if condition.RatingCondition != nil {
				newCondition.RatingCondition = &mondoov1.WorkspaceSelectionRatingConditionInput{
					Field:    mondoov1.WorkspaceSelectionConditionRatingField(condition.RatingCondition.Field.ValueString()),
					Operator: mondoov1.WorkspaceSelectionConditionRatingOperator(condition.RatingCondition.Operator.ValueString()),
					Values:   ConvertSlice[mondoov1.ScoreRating](condition.RatingCondition.Values),
				}
			}
			newSelection.Conditions = append(newSelection.Conditions, newCondition)
		}
		selectionInput.Selections = append(selectionInput.Selections, newSelection)
	}
	return selectionInput
}

type Workspace struct {
	Mrn         string              `json:"mrn"`
	OwnerMrn    string              `json:"ownerMrn"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Selections  WorkspaceSelections `json:"selections"`
}
type WorkspaceSelections struct {
	Selections []WorkspaceSelection `json:"selections"`
}
type WorkspaceSelection struct {
	Conditions []WorkspaceCondition `json:"conditions"`
}
type WorkspaceCondition struct {
	Operator  mondoov1.WorkspaceSelectionConditionOperator
	Condition Condition
}
type Condition struct {
	Typename          mondoov1.String                     `graphql:"__typename"`
	StringCondition   WorkspaceSelectionStringCondition   `graphql:"... on WorkspaceSelectionStringCondition"`
	IntCondition      WorkspaceSelectionIntCondition      `graphql:"... on WorkspaceSelectionIntCondition"`
	RatingCondition   WorkspaceSelectionRatingCondition   `graphql:"... on WorkspaceSelectionRatingCondition"`
	KeyValueCondition WorkspaceSelectionKeyValueCondition `graphql:"... on WorkspaceSelectionKeyValueCondition"`
}
type WorkspaceSelectionKeyValueCondition struct {
	Field    mondoov1.WorkspaceSelectionConditionKeyValueField    `graphql:"keyValueField: field"`
	Operator mondoov1.WorkspaceSelectionConditionKeyValueOperator `graphql:"keyValueOperator: operator"`
	Values   []KeyValue                                           `graphql:"keyValueValues: values"`
}
type WorkspaceSelectionStringCondition struct {
	Field    mondoov1.WorkspaceSelectionConditionStringField    `graphql:"stringField: field"`
	Operator mondoov1.WorkspaceSelectionConditionStringOperator `graphql:"stringOperator: operator"`
	Values   []WorkspaceSelectionStringConditionValue           `graphql:"stringValues: values"`
}

func (c WorkspaceSelectionStringCondition) ValuesStringSlice() []string {
	slice := make([]string, len(c.Values))
	for i, v := range c.Values {
		slice[i] = v.Value
	}
	return slice
}

type WorkspaceSelectionStringConditionValue struct {
	Value string
}
type WorkspaceSelectionIntCondition struct {
	Field    mondoov1.WorkspaceSelectionConditionIntField        `graphql:"intField: field"`
	Operator mondoov1.WorkspaceSelectionConditionNumericOperator `graphql:"intOperator: operator"`
	Values   []int32                                             `graphql:"intValues: values"`
}
type WorkspaceSelectionRatingCondition struct {
	Field    mondoov1.WorkspaceSelectionConditionRatingField    `graphql:"ratingField: field"`
	Operator mondoov1.WorkspaceSelectionConditionRatingOperator `graphql:"ratingOperator: operator"`
	Values   []string                                           `graphql:"ratingValues: values"`
}

func (r *WorkspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkspaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	// Do GraphQL request to API to create the resource

	createInput := mondoov1.CreateWorkspaceInput{
		OwnerMrn:    mondoov1.String(space.MRN()),
		Name:        mondoov1.String(data.Name.ValueString()),
		Description: mondoov1.NewStringPtr(mondoov1.String(data.Description.ValueString())),
		Selections:  renderSelectionsFromModel(&data),
	}

	tflog.Debug(ctx, "CreateWorkspaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createInput),
	})

	var createMutation struct {
		Workspace struct {
			Workspace
		} `graphql:"createWorkspace(input: $input)"`
	}

	err = r.client.Mutate(ctx, &createMutation, createInput, nil)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create workspace. Got error: %s", err),
			)
		return
	}

	// Write logs using the tflog package
	tflog.Debug(ctx, "created workspace", map[string]interface{}{
		"response": fmt.Sprintf("%+v", createMutation),
	})
	// Save space mrn into the Terraform state.
	data.SpaceID = types.StringValue(SpaceFrom(createMutation.Workspace.OwnerMrn).ID())
	data.Mrn = types.StringValue(createMutation.Workspace.Mrn)
	data.Name = types.StringValue(createMutation.Workspace.Name)
	data.Description = types.StringValue(createMutation.Workspace.Description)
	data.Selections = renderSelectionsFromGraphql(createMutation.Workspace.Selections)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceResource) queryWorkspace(ctx context.Context, mrn string) (WorkspaceResourceModel, error) {
	var q struct {
		Workspace struct {
			Workspace `graphql:"... on Workspace"`
		} `graphql:"workspace(mrn: $mrn)"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}

	tflog.Debug(ctx, "workspaceQueryVariables", map[string]interface{}{
		"input": fmt.Sprintf("%+v", variables),
	})

	err := r.client.Query(ctx, &q, variables)
	if err != nil {
		return WorkspaceResourceModel{}, err
	}

	tflog.Debug(ctx, "workspaceResponse", map[string]interface{}{
		"payload": fmt.Sprintf("%+v", q),
	})

	return WorkspaceResourceModel{
		SpaceID:     types.StringValue(SpaceFrom(q.Workspace.OwnerMrn).ID()),
		Mrn:         types.StringValue(q.Workspace.Mrn),
		Name:        types.StringValue(q.Workspace.Name),
		Description: types.StringValue(q.Workspace.Description),
		Selections:  renderSelectionsFromGraphql(q.Workspace.Selections),
	}, nil
}

func (r *WorkspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkspaceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	m, err := r.queryWorkspace(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to read workspace. Got error: %s", err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// Update is not allowed by design. We only read and exist.
func (r *WorkspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WorkspaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	// Do GraphQL request to API to create the resource

	selections := renderSelectionsFromModel(&data)
	updateInput := mondoov1.UpdateWorkspaceInput{
		Mrn:         mondoov1.String(data.Mrn.ValueString()),
		Name:        mondoov1.NewStringPtr(mondoov1.String(data.Name.ValueString())),
		Description: mondoov1.NewStringPtr(mondoov1.String(data.Description.ValueString())),
		Selections:  &selections,
	}

	tflog.Debug(ctx, "CreateWorkspaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", updateInput),
	})

	var createMutation struct {
		Workspace struct {
			Workspace
		} `graphql:"updateWorkspace(input: $input)"`
	}

	err = r.client.Mutate(ctx, &createMutation, updateInput, nil)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update workspace. Got error: %s", err),
			)
		return
	}

	// Write logs using the tflog package
	tflog.Debug(ctx, "updated workspace", map[string]interface{}{
		"response": fmt.Sprintf("%+v", createMutation),
	})
	// Save space mrn into the Terraform state.
	data.SpaceID = types.StringValue(SpaceFrom(createMutation.Workspace.OwnerMrn).ID())
	data.Mrn = types.StringValue(createMutation.Workspace.Mrn)
	data.Name = types.StringValue(createMutation.Workspace.Name)
	data.Description = types.StringValue(createMutation.Workspace.Description)
	data.Selections = renderSelectionsFromGraphql(createMutation.Workspace.Selections)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkspaceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to delete the resource.
	var deleteMutation struct {
		DeleteWorkspace mondoov1.Boolean `graphql:"deleteWorkspaces(input: $input)"`
	}

	input := mondoov1.DeleteWorkspacesInput{
		Mrns: []mondoov1.String{mondoov1.String(data.Mrn.ValueString())},
	}
	tflog.Debug(ctx, "DeleteWorkspaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", input),
	})
	err := r.client.Mutate(ctx, &deleteMutation, input, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to delete workspace. Got error: %s", err),
		)
	}
}

func (r *WorkspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := req.ID

	m, err := r.queryWorkspace(ctx, mrn)
	if err != nil {
		resp.
			Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to import workspace. Got error: %s", err,
				),
			)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// We need to have a way to combine all enums into a single array.
// TODO add it to mondoo-go.
func displayPossibleStringOperators() []mondoov1.WorkspaceSelectionConditionStringOperator {
	return []mondoov1.WorkspaceSelectionConditionStringOperator{
		mondoov1.WorkspaceSelectionConditionStringOperatorEqual,
		mondoov1.WorkspaceSelectionConditionStringOperatorNotEqual,
		mondoov1.WorkspaceSelectionConditionStringOperatorContains,
	}
}
func displayPossibleNumericOperators() []mondoov1.WorkspaceSelectionConditionNumericOperator {
	return []mondoov1.WorkspaceSelectionConditionNumericOperator{
		mondoov1.WorkspaceSelectionConditionNumericOperatorEqual,
		mondoov1.WorkspaceSelectionConditionNumericOperatorNotEqual,
		mondoov1.WorkspaceSelectionConditionNumericOperatorGt,
		mondoov1.WorkspaceSelectionConditionNumericOperatorLt,
	}
}
func displayPossibleRatingOperators() []mondoov1.WorkspaceSelectionConditionRatingOperator {
	return []mondoov1.WorkspaceSelectionConditionRatingOperator{
		mondoov1.WorkspaceSelectionConditionRatingOperatorEqual,
		mondoov1.WorkspaceSelectionConditionRatingOperatorNotEqual,
	}
}
func displayPossibleKeyValueOperators() []mondoov1.WorkspaceSelectionConditionKeyValueOperator {
	return []mondoov1.WorkspaceSelectionConditionKeyValueOperator{
		mondoov1.WorkspaceSelectionConditionKeyValueOperatorContains,
	}
}
func displayPossibleStringFields() []mondoov1.WorkspaceSelectionConditionStringField {
	return []mondoov1.WorkspaceSelectionConditionStringField{
		mondoov1.WorkspaceSelectionConditionStringFieldPlatform,
		mondoov1.WorkspaceSelectionConditionStringFieldPlatformVersion,
		mondoov1.WorkspaceSelectionConditionStringFieldAssetName,
		mondoov1.WorkspaceSelectionConditionStringFieldAssetKind,
		mondoov1.WorkspaceSelectionConditionStringFieldTechnology,
	}
}
func displayPossibleIntFields() []mondoov1.WorkspaceSelectionConditionIntField {
	return []mondoov1.WorkspaceSelectionConditionIntField{
		mondoov1.WorkspaceSelectionConditionIntFieldRiskScore,
	}
}
func displayPossibleRatingFields() []mondoov1.WorkspaceSelectionConditionRatingField {
	return []mondoov1.WorkspaceSelectionConditionRatingField{
		mondoov1.WorkspaceSelectionConditionRatingFieldRisk,
	}
}
func displayPossibleKeyValueFields() []mondoov1.WorkspaceSelectionConditionKeyValueField {
	return []mondoov1.WorkspaceSelectionConditionKeyValueField{
		mondoov1.WorkspaceSelectionConditionKeyValueFieldLabels,
		mondoov1.WorkspaceSelectionConditionKeyValueFieldAnnotations,
	}
}
