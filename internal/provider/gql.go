// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

const orgPrefix = "//captain.api.mondoo.app/organizations/"

// The extended GraphQL client allows us to pass additional information to
// resources and data sources, things like the Mondoo space.
type ExtendedGqlClient struct {
	*mondoov1.Client

	// The default space configured at the provider level, if configured, all resources
	// will be managed there unless the resource itself specifies a different space
	space Space
}

// Space returns the space configured into the extended GraphQL client.
func (c *ExtendedGqlClient) Space() Space {
	return c.space
}

// ComputeSpace receives an optional space ID, if it is empty, it tries to return the space
// configured into the exptended client, but if both are empty, it throws an error.
func (c *ExtendedGqlClient) ComputeSpace(spaceID types.String) (Space, error) {
	if spaceID.ValueString() != "" {
		return Space(spaceID.ValueString()), nil
	}

	if c.space != "" {
		return c.space, nil
	}
	return c.space, errors.New("no space configured on either resource or provider blocks")
}

// newDataUrl generates a https://tools.ietf.org/html/rfc2397 data url for a given content.
func newDataUrl(content []byte) string {
	return "data:application/x-yaml;base64," + base64.StdEncoding.EncodeToString(content)
}

type createSpacePayload struct {
	Id   mondoov1.ID
	Mrn  mondoov1.String
	Name mondoov1.String
}

func (c *ExtendedGqlClient) CreateSpace(ctx context.Context, orgID string, id string, name string) (createSpacePayload, error) {
	var createMutation struct {
		CreateSpace createSpacePayload `graphql:"createSpace(input: $input)"`
	}

	var spaceID *mondoov1.String
	if id != "" {
		spaceID = mondoov1.NewStringPtr(mondoov1.String(id))
	}

	createInput := mondoov1.CreateSpaceInput{
		Name:   mondoov1.String(name),
		ID:     spaceID,
		OrgMrn: mondoov1.String(orgPrefix + orgID),
	}

	tflog.Trace(ctx, "CreateSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createInput),
	})

	err := c.Mutate(ctx, &createMutation, createInput, nil)
	return createMutation.CreateSpace, err
}

func (c *ExtendedGqlClient) UpdateSpace(ctx context.Context, spaceID string, name string) error {
	var updateMutation struct {
		UpdateSpace struct {
			Space struct {
				Mrn  mondoov1.String
				Name mondoov1.String
			}
		} `graphql:"updateSpace(input: $input)"`
	}
	updateInput := mondoov1.UpdateSpaceInput{
		Mrn:  mondoov1.String(spacePrefix + spaceID),
		Name: mondoov1.String(name),
	}
	tflog.Trace(ctx, "UpdateSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", updateInput),
	})
	return c.Mutate(ctx, &updateMutation, updateInput, nil)
}

func (c *ExtendedGqlClient) DeleteSpace(ctx context.Context, spaceID string) error {
	var deleteMutation struct {
		DeleteSpace mondoov1.String `graphql:"deleteSpace(spaceMrn: $spaceMrn)"`
	}
	variables := map[string]interface{}{
		"spaceMrn": mondoov1.ID(spacePrefix + spaceID),
	}

	tflog.Trace(ctx, "DeleteSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", variables),
	})

	return c.Mutate(ctx, &deleteMutation, nil, variables)
}

type spacePayload struct {
	Id           string
	Mrn          string
	Name         string
	Organization orgPayload
}

func (c *ExtendedGqlClient) GetSpace(ctx context.Context, mrn string) (spacePayload, error) {
	var q struct {
		Space spacePayload `graphql:"space(mrn: $mrn)"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return spacePayload{}, err
	}

	return q.Space, nil
}

type orgPayload struct {
	Id   string
	Mrn  string
	Name string
}

func (c *ExtendedGqlClient) GetOrganization(ctx context.Context, mrn string) (orgPayload, error) {
	var q struct {
		Organization orgPayload `graphql:"organization(mrn: $mrn)"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return orgPayload{}, err
	}

	return q.Organization, nil
}

type setCustomPolicyPayload struct {
	PolicyMrns []mondoov1.String
}

func (c *ExtendedGqlClient) SetCustomPolicy(ctx context.Context, scopeMrn string, overwriteVal *bool, policyBundleData []byte) (setCustomPolicyPayload, error) {
	var overwrite *mondoov1.Boolean
	if overwriteVal != nil {
		overwrite = mondoov1.NewBooleanPtr(mondoov1.Boolean(*overwriteVal))
	}

	setCustomPolicyInput := mondoov1.SetCustomPolicyInput{
		SpaceMrn:  mondoov1.NewStringPtr(mondoov1.String(scopeMrn)),
		Overwrite: overwrite,
		Dataurl:   mondoov1.String(newDataUrl(policyBundleData)),
	}

	var setCustomPolicy struct {
		SetCustomPolicyPayload setCustomPolicyPayload `graphql:"setCustomPolicy(input: $input)"`
	}

	err := c.Mutate(ctx, &setCustomPolicy, []mondoov1.SetCustomPolicyInput{setCustomPolicyInput}, nil)
	return setCustomPolicy.SetCustomPolicyPayload, err
}

type SpaceReportInput struct {
	SpaceMrn mondoov1.String
}

type Policy struct {
	Mrn       mondoov1.String
	Name      mondoov1.String
	Assigned  mondoov1.Boolean
	Action    mondoov1.String
	Version   mondoov1.String
	IsPublic  mondoov1.Boolean
	CreatedAt mondoov1.String
	UpdatedAt mondoov1.String
	Docs      mondoov1.String
}

type PolicyNode struct {
	Policy Policy
}

type PolicyEdge struct {
	Cursor mondoov1.String
	Node   PolicyNode
}

type PolicyReportSummaries struct {
	TotalCount int
	Edges      []PolicyEdge
}

type SpaceReport struct {
	SpaceMrn              mondoov1.String
	PolicyReportSummaries PolicyReportSummaries
}

type SpaceReportPayload struct {
	SpaceReport SpaceReport
}

type ContentInput struct {
	ScopeMrn     string
	CatalogType  string
	AssignedOnly bool
}

type Node struct {
	Policy Policy `graphql:"... on Policy"`
}

type Edge struct {
	Node Node
}

type Content struct {
	TotalCount int
	Edges      []Edge
}

type ContentPayload struct {
	Content Content
}

func (c *ExtendedGqlClient) DownloadBundle(ctx context.Context, policyMrn string) (string, error) {
	var q struct {
		DownloadBundle struct {
			PolicyBundleYaml struct {
				Yaml string `graphql:"yaml"`
			} `graphql:"... on PolicyBundleYaml"`
		} `graphql:"downloadBundle(input: $input)"`
	}
	variables := map[string]interface{}{
		"input": mondoov1.DownloadBundleInput{
			Mrn: mondoov1.String(policyMrn),
		},
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return "", err
	}

	return q.DownloadBundle.PolicyBundleYaml.Yaml, nil
}

func (c *ExtendedGqlClient) GetPolicies(ctx context.Context, scopeMrn string, catalogType string, assignedOnly bool) (*[]Policy, error) {
	// Define the query struct according to the provided query
	var contentQuery struct {
		Content Content `graphql:"content(input: $input)"`
	}
	// Define the input variable according to the provided query
	input := mondoov1.ContentSearchInput{
		ScopeMrn:     mondoov1.String(scopeMrn),
		CatalogType:  mondoov1.CatalogType(catalogType),
		AssignedOnly: mondoov1.NewBooleanPtr(mondoov1.Boolean(assignedOnly)),
		Limit:        mondoov1.NewIntPtr(mondoov1.Int(10000)),
	}

	variables := map[string]interface{}{
		"input": input,
	}

	tflog.Trace(ctx, "GetContentInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", input),
	})

	// Execute the query
	err := c.Query(ctx, &contentQuery, variables)
	if err != nil {
		return nil, err
	}

	var policies []Policy
	for _, edges := range contentQuery.Content.Edges {
		policies = append(policies, edges.Node.Policy)
	}

	return &policies, nil
}

func (c *ExtendedGqlClient) GetPolicy(ctx context.Context, policyMrn string, spaceMrn string) (*Policy, error) {
	var q struct {
		Policy Policy `graphql:"policy(input: $input)"`
	}

	input := mondoov1.PolicyInput{
		Mrn:      mondoov1.NewStringPtr(mondoov1.String(policyMrn)),
		SpaceMrn: mondoov1.NewStringPtr(mondoov1.String(spaceMrn)),
	}

	variables := map[string]interface{}{
		"input": input,
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return nil, err
	}

	return &q.Policy, nil
}

func (c *ExtendedGqlClient) AssignPolicy(ctx context.Context, spaceMrn string, action mondoov1.PolicyAction, policyMrns []string) error {
	var list *[]mondoov1.String

	entries := []mondoov1.String{}
	for i := range policyMrns {
		entries = append(entries, mondoov1.String(policyMrns[i]))
	}

	if len(entries) > 0 {
		list = &entries
	}

	policyAssignmentInput := mondoov1.PolicyAssignmentInput{
		AssetMrn:   mondoov1.String(spaceMrn),
		PolicyMrns: list,
		Action:     &action,
	}

	var policyAssignment struct {
		AssignPolicy bool `graphql:"assignPolicy(input: $input)"`
	}

	return c.Mutate(ctx, &policyAssignment, policyAssignmentInput, nil)
}

func (c *ExtendedGqlClient) UnassignPolicy(ctx context.Context, spaceMrn string, policyMrns []string) error {
	var list *[]mondoov1.String

	entries := []mondoov1.String{}
	for i := range policyMrns {
		entries = append(entries, mondoov1.String(policyMrns[i]))
	}

	if len(entries) > 0 {
		list = &entries
	}

	policyAssignmentInput := mondoov1.PolicyAssignmentInput{
		AssetMrn:   mondoov1.String(spaceMrn),
		PolicyMrns: list,
	}

	var policyAssignment struct {
		AssignPolicy bool `graphql:"unassignPolicy(input: $input)"`
	}

	return c.Mutate(ctx, &policyAssignment, policyAssignmentInput, nil)
}

type SetCustomPolicyPayload struct {
	QueryPackMrns []mondoov1.String
}

func (c *ExtendedGqlClient) SetCustomQueryPack(ctx context.Context, scopeMrn string, overwriteVal *bool, policyBundleData []byte) (SetCustomPolicyPayload, error) {
	var overwrite *mondoov1.Boolean
	if overwriteVal != nil {
		overwrite = mondoov1.NewBooleanPtr(mondoov1.Boolean(*overwriteVal))
	}

	setCustomPolicyInput := mondoov1.SetCustomQueryPackInput{
		SpaceMrn:  mondoov1.String(scopeMrn),
		Overwrite: overwrite,
		Dataurl:   mondoov1.String(newDataUrl(policyBundleData)),
	}

	var setCustomQueryPackPayload struct {
		SetCustomPolicyPayload SetCustomPolicyPayload `graphql:"setCustomQueryPack(input: $input)"`
	}

	err := c.Mutate(ctx, &setCustomQueryPackPayload, []mondoov1.SetCustomQueryPackInput{setCustomPolicyInput}, nil)
	return setCustomQueryPackPayload.SetCustomPolicyPayload, err
}

func (c *ExtendedGqlClient) DeletePolicy(ctx context.Context, policyMrn string) error {
	deleteCustomPolicyInput := mondoov1.DeleteCustomPolicyInput{
		PolicyMrn: mondoov1.String(policyMrn),
	}

	var deleteCustomPolicy struct {
		DeleteCustomPolicyPayload struct {
			PolicyMrn mondoov1.String
		} `graphql:"deleteCustomPolicy(input: $input)"`
	}

	return c.Mutate(ctx, &deleteCustomPolicy, deleteCustomPolicyInput, nil)
}

type CreateClientIntegrationPayload struct {
	Mrn   mondoov1.String
	Name  mondoov1.String
	Token mondoov1.String
}

func (c *ExtendedGqlClient) CreateIntegration(ctx context.Context, spaceMrn, name string, typ mondoov1.ClientIntegrationType, opts mondoov1.ClientIntegrationConfigurationInput) (*CreateClientIntegrationPayload, error) {
	var createMutation struct {
		CreateClientIntegration struct {
			Integration CreateClientIntegrationPayload
		} `graphql:"createClientIntegration(input: $input)"`
	}

	createInput := mondoov1.CreateClientIntegrationInput{
		SpaceMrn:             mondoov1.String(spaceMrn),
		Name:                 mondoov1.String(name),
		Type:                 typ,
		LongLivedToken:       false,
		ConfigurationOptions: opts,
	}

	tflog.Trace(ctx, "CreateClientIntegrationInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createInput),
	})

	err := c.Mutate(ctx, &createMutation, createInput, nil)
	if err != nil {
		return nil, err
	}
	return &createMutation.CreateClientIntegration.Integration, nil
}

type GetClientIntegrationTokenInput struct {
	mrn            mondoov1.String
	longLivedToken mondoov1.Boolean
}

type ClientIntegrationToken struct {
	Token mondoov1.String
}

func (c *ExtendedGqlClient) GetClientIntegrationToken(ctx context.Context, mrn string, longLivedToken bool) (*ClientIntegrationToken, error) {
	// Define the response structure
	var query struct {
		ClientIntegrationToken ClientIntegrationToken `graphql:"getClientIntegrationToken(input: $input)"`
	}

	// Define the input variables
	input := GetClientIntegrationTokenInput{
		mrn:            mondoov1.String(mrn),
		longLivedToken: mondoov1.Boolean(longLivedToken),
	}
	variables := map[string]interface{}{
		"input": input,
	}

	// Trace the input variables for debugging
	tflog.Trace(ctx, "GetClientIntegrationTokenInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", input),
	})

	// Perform the GraphQL query
	err := c.Query(ctx, &query, variables)
	if err != nil {
		return nil, err
	}

	// Return the token from the response
	return &query.ClientIntegrationToken, nil
}

type UpdateIntegrationPayload struct {
	Name mondoov1.String
}

func (c *ExtendedGqlClient) UpdateIntegration(ctx context.Context, mrn, name string, typ mondoov1.ClientIntegrationType, opts mondoov1.ClientIntegrationConfigurationInput) (*UpdateIntegrationPayload, error) {
	var updateMutation struct {
		UpdateIntegrationPayload `graphql:"updateClientIntegrationConfiguration(input: $input)"`
	}

	updateInput := mondoov1.UpdateClientIntegrationConfigurationInput{
		Mrn:                  mondoov1.String(mrn),
		Name:                 mondoov1.NewStringPtr(mondoov1.String(name)),
		Type:                 typ,
		ConfigurationOptions: opts,
	}
	tflog.Trace(ctx, "UpdateClientIntegrationNameInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", updateInput),
	})
	err := c.Mutate(ctx, &updateMutation, updateInput, nil)
	if err != nil {
		return nil, err
	}
	return &updateMutation.UpdateIntegrationPayload, nil
}

type DeleteIntegrationPayload struct {
	Mrn mondoov1.String
}

func (c *ExtendedGqlClient) DeleteIntegration(ctx context.Context, mrn string) (*DeleteIntegrationPayload, error) {
	var deleteMutation struct {
		DeleteClientIntegration DeleteIntegrationPayload `graphql:"deleteClientIntegration(input: $input)"`
	}
	deleteInput := mondoov1.DeleteClientIntegrationInput{
		Mrn: mondoov1.String(mrn),
	}
	tflog.Trace(ctx, "DeleteClientIntegration", map[string]interface{}{
		"input": fmt.Sprintf("%+v", deleteInput),
	})
	err := c.Mutate(ctx, &deleteMutation, deleteInput, nil)
	if err != nil {
		return nil, err
	}
	return &deleteMutation.DeleteClientIntegration, nil
}

type AzureConfigurationOptions struct {
	TenantId               string
	ClientId               string
	SubscriptionsWhitelist []string
	SubscriptionsBlacklist []string
	ScanVms                bool
}

type HostConfigurationOptions struct {
	Host  string `graphql:"host"`
	HTTPS bool   `graphql:"https"`
	HTTP  bool   `graphql:"http"`
}

type SlackConfigurationOptions struct {
	Placeholder string
}

type GithubConfigurationOptions struct {
	Owner          string
	Repository     string
	Organization   string
	ReposAllowList []string
	ReposDenyList  []string
}

type GitlabConfigurationOptions struct {
	Group                string
	DiscoverGroups       bool
	DiscoverProjects     bool
	DiscoverTerraform    bool
	DiscoverK8sManifests bool
	BaseURL              string
}

type Ms365ConfigurationOptions struct {
	TenantId string
	ClientId string
}

type HostedAwsConfigurationOptions struct {
	AccessKeyId string
	Role        string
}

type GcpConfigurationOptions struct {
	ProjectId   string
	DiscoverAll bool
}

type ShodanConfigurationOptions struct {
	Targets []string
}

type ZendeskConfigurationOptions struct {
	Subdomain         string
	Email             string
	AutoCloseTickets  bool
	AutoCreateTickets bool
	CustomFields      []ZendeskCustomField
}

type ZendeskCustomField struct {
	ID    int64
	Value string
}

type JiraConfigurationOptions struct {
	Host             string
	Email            string
	DefaultProject   string
	AutoCloseTickets bool
	AutoCreateCases  bool
}

type EmailConfigurationOptions struct {
	Recipients        []EmailRecipient
	AutoCreateTickets bool
	AutoCloseTickets  bool
}

type EmailRecipient struct {
	Name         string
	Email        string
	IsDefault    bool
	ReferenceURL string
}

type MicrosoftDefenderConfigurationOptionsInput struct {
	TenantId               string
	ClientId               string
	SubscriptionsAllowlist []string
	SubscriptionsDenylist  []string
}
type CrowdstrikeFalconConfigurationOptionsInput struct {
	ClientId  string
	Cloud     string
	MemberCID string
}
type SentinelOneConfigurationOptions struct {
	Host    string
	Account string
}

type ClientIntegrationConfigurationOptions struct {
	AzureConfigurationOptions                  AzureConfigurationOptions                  `graphql:"... on AzureConfigurationOptions"`
	HostConfigurationOptions                   HostConfigurationOptions                   `graphql:"... on HostConfigurationOptions"`
	Ms365ConfigurationOptions                  Ms365ConfigurationOptions                  `graphql:"... on Ms365ConfigurationOptions"`
	GcpConfigurationOptions                    GcpConfigurationOptions                    `graphql:"... on GcpConfigurationOptions"`
	SlackConfigurationOptions                  SlackConfigurationOptions                  `graphql:"... on SlackConfigurationOptions"`
	GithubConfigurationOptions                 GithubConfigurationOptions                 `graphql:"... on GithubConfigurationOptions"`
	HostedAwsConfigurationOptions              HostedAwsConfigurationOptions              `graphql:"... on HostedAwsConfigurationOptions"`
	ShodanConfigurationOptions                 ShodanConfigurationOptions                 `graphql:"... on ShodanConfigurationOptions"`
	ZendeskConfigurationOptions                ZendeskConfigurationOptions                `graphql:"... on ZendeskConfigurationOptions"`
	JiraConfigurationOptions                   JiraConfigurationOptions                   `graphql:"... on JiraConfigurationOptions"`
	EmailConfigurationOptions                  EmailConfigurationOptions                  `graphql:"... on EmailConfigurationOptions"`
	GitlabConfigurationOptions                 GitlabConfigurationOptions                 `graphql:"... on GitlabConfigurationOptions"`
	MicrosoftDefenderConfigurationOptionsInput MicrosoftDefenderConfigurationOptionsInput `graphql:"... on MicrosoftDefenderConfigurationOptions"`
	CrowdstrikeFalconConfigurationOptionsInput CrowdstrikeFalconConfigurationOptionsInput `graphql:"... on CrowdstrikeFalconConfigurationOptions"`
	SentinelOneConfigurationOptions            SentinelOneConfigurationOptions            `graphql:"... on CrowdstrikeFalconConfigurationOptions"`
	// Add other configuration options here
}

type Integration struct {
	Mrn                  string
	Name                 string
	ConfigurationOptions ClientIntegrationConfigurationOptions `graphql:"configurationOptions"`
}

// SpaceID returns the space where the integration is configured (using the integration MRN).
func (i Integration) SpaceID() string {
	// we are expecting MRNs like:
	// => "//captain.api.mondoo.app/spaces/{ID}/integrations/{ID}"
	mrnSplit := strings.Split(i.Mrn, "/")
	l := len(mrnSplit)
	if l >= 3 { // check for safety
		return mrnSplit[l-3]
	}
	return ""
}

type ClientIntegration struct {
	Integration Integration
}

func (c *ExtendedGqlClient) GetClientIntegration(ctx context.Context, mrn string) (Integration, error) {
	var q struct {
		ClientIntegration ClientIntegration `graphql:"clientIntegration(input: {mrn: $mrn})"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return Integration{}, err
	}

	return q.ClientIntegration.Integration, nil
}

type triggerActionPayload struct {
	Mrn string
}

func (c *ExtendedGqlClient) TriggerAction(ctx context.Context, integrationMrn string, action mondoov1.ActionType) (triggerActionPayload, error) {

	var q struct {
		TriggerAction triggerActionPayload `graphql:"triggerAction(input: { mrn: $mrn, type: $type })"`
	}
	variables := map[string]interface{}{
		"mrn":  mondoov1.String(integrationMrn),
		"type": action,
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return triggerActionPayload{}, err
	}

	return q.TriggerAction, nil
}

type AssetScore struct {
	Grade string
	Value int64
}

type KeyValue struct {
	Key   string
	Value string
}

type AssetNode struct {
	Id           string
	Mrn          string
	State        string
	Name         string
	UpdatedAt    string
	ReferenceIDs []string `graphql:"referenceIDs"`
	Asset_type   string
	Score        AssetScore
	Annotations  []KeyValue
}

type AssetEdge struct {
	Cursor string
	Node   AssetNode
}

type AssetsPayload struct {
	TotalCount int
	Edges      []AssetEdge
}

func (c *ExtendedGqlClient) GetAssets(ctx context.Context, spaceMrn string) (AssetsPayload, error) {
	var q struct {
		Assets AssetsPayload `graphql:"assets(spaceMrn: $spaceMrn)"`
	}
	variables := map[string]interface{}{
		"spaceMrn": mondoov1.String(spaceMrn),
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return AssetsPayload{}, err
	}

	return q.Assets, nil
}

func (c *ExtendedGqlClient) SetScimGroupMapping(ctx context.Context, orgMrn string, group string, mappings []mondoov1.ScimGroupMapping) error {
	var setScimGroupMappingMutation struct {
		SetScimGroupMapping struct {
			Group mondoov1.String
		} `graphql:"setScimGroupMapping(input: $input)"`
	}

	setScimGroupMappingInput := mondoov1.SetScimGroupMappingInput{
		OrgMrn:   mondoov1.String(orgMrn),
		Group:    mondoov1.String(group),
		Mappings: mappings,
	}

	return c.Mutate(ctx, &setScimGroupMappingMutation, setScimGroupMappingInput, nil)
}

func (c *ExtendedGqlClient) UploadFramework(ctx context.Context, spaceMrn string, content []byte) error {
	// Define the mutation struct according to the provided query
	var uploadMutation struct {
		UploadFramework bool `graphql:"uploadFramework(input: $input)"`
	}

	// Define the input variable according to the provided query
	input := mondoov1.UploadFrameworkInput{
		SpaceMrn: mondoov1.String(spaceMrn),
		Dataurl:  mondoov1.String(newDataUrl(content)),
	}

	// Execute the mutation
	return c.Mutate(ctx, &uploadMutation, input, nil)
}

type ComplianceFrameworkPayload struct {
	Mrn      mondoov1.String
	Name     mondoov1.String
	State    mondoov1.String
	ScopeMrn mondoov1.String
}

func (c *ExtendedGqlClient) GetFramework(ctx context.Context, spaceMrn string, spaceId string, uid string) (*ComplianceFrameworkPayload, error) {
	// Define the query struct according to the provided query
	var getFrameworkQuery struct {
		ComplianceFramework ComplianceFrameworkPayload `graphql:"complianceFramework(input: $input)"`
	}
	frameworkMrn := fmt.Sprintf("//policy.api.mondoo.app/spaces/%s/frameworks/%s", spaceId, uid)
	// Define the input variable according to the provided query
	input := mondoov1.ComplianceFrameworkInput{
		ScopeMrn:     mondoov1.String(spaceMrn),
		FrameworkMrn: mondoov1.String(frameworkMrn),
	}

	variables := map[string]interface{}{
		"input": input,
	}

	// Execute the query
	err := c.Query(ctx, &getFrameworkQuery, variables)
	if err != nil {
		return nil, err
	}

	return &getFrameworkQuery.ComplianceFramework, nil
}

type ComplianceFrameworksPayload struct {
	Authors                  []Author `graphql:"authors"`
	Completion               mondoov1.Float
	Description              mondoov1.String
	Mrn                      mondoov1.String
	Name                     mondoov1.String
	PreviousCompletionScores PreviousCompletionScores `graphql:"previousCompletionScores"`
	ScopeMrn                 mondoov1.String
	State                    mondoov1.String
	Summary                  mondoov1.String
	Tags                     []Tag `graphql:"tags"`
}

type Author struct {
	Name  mondoov1.String
	Email mondoov1.String
}

type PreviousCompletionScores struct {
	Entries []Entry `graphql:"entries"`
}

type Entry struct {
	Score     mondoov1.Float
	Timestamp mondoov1.String
}

type Tag struct {
	Key   mondoov1.String
	Value mondoov1.String
}

type GetComplianceFrameworksQuery struct {
	ComplianceFrameworks []ComplianceFrameworksPayload `graphql:"complianceFrameworks(input: $input)"`
}

func (c *ExtendedGqlClient) ListFrameworks(ctx context.Context, scopeMrn string) ([]ComplianceFrameworksPayload, error) {
	// Define the query struct according to the provided query
	var getFrameworksQuery GetComplianceFrameworksQuery

	// Define the input variable according to the provided query
	input := mondoov1.ComplianceFrameworksInput{
		ScopeMrn: mondoov1.String(scopeMrn),
	}

	variables := map[string]interface{}{
		"input": input,
	}

	// Execute the query
	err := c.Query(ctx, &getFrameworksQuery, variables)
	if err != nil {
		return nil, err
	}

	return getFrameworksQuery.ComplianceFrameworks, nil
}

func (c *ExtendedGqlClient) UpdateFramework(ctx context.Context, frameworkMrn string, scopeMrn string, enabled bool) error {
	var updateMutation struct {
		ApplyFramework bool `graphql:"applyFrameworkMutation(input: $input)"`
	}

	input := mondoov1.ComplianceFrameworkMutationInput{
		FrameworkMrn: mondoov1.String(frameworkMrn),
		ScopeMrn:     mondoov1.String(scopeMrn),
	}

	if enabled {
		input.Action = mondoov1.ComplianceFrameworkMutationActionEnable
	} else {
		input.Action = mondoov1.ComplianceFrameworkMutationActionPreview
	}

	return c.Mutate(ctx, &updateMutation, input, nil)
}

func (c *ExtendedGqlClient) BulkUpdateFramework(ctx context.Context, frameworkMrns basetypes.ListValue, spaceId string, enabled bool) error {
	scopeMrn := ""
	if spaceId != "" {
		scopeMrn = spacePrefix + spaceId
	}

	var frameworkList []mondoov1.String
	listFrameworks, _ := frameworkMrns.ToListValue(ctx)
	listFrameworks.ElementsAs(ctx, &frameworkList, true)

	for _, mrn := range frameworkList {
		err := c.UpdateFramework(ctx, string(mrn), scopeMrn, enabled)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ExtendedGqlClient) DeleteFramework(ctx context.Context, mrn string) error {
	// Define the mutation struct according to the provided query
	var deleteMutation struct {
		DeleteFramework bool `graphql:"deleteFramework(input: $input)"`
	}

	// Define the input variable according to the provided query
	input := mondoov1.DeleteFrameworkInput{
		Mrn: mondoov1.String(mrn),
	}

	// Execute the mutation
	return c.Mutate(ctx, &deleteMutation, input, nil)
}

// ImportIntegration is a generic way to import an integration, this function fetches the integration from
// the provided MRN and if it exists, it compares the space configured at the provider level (if any).
func (c *ExtendedGqlClient) ImportIntegration(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) (*Integration, bool) {
	mrn := req.ID
	ctx = tflog.SetField(ctx, "mrn", mrn)
	tflog.Debug(ctx, "importing integration")
	integration, err := c.GetClientIntegration(ctx, mrn)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to get integration. Got error: %s", err),
			)
		return nil, false
	}

	spaceID := integration.SpaceID()
	if c.Space().ID() != "" && c.Space().ID() != spaceID {
		// The provider is configured to manage resources in a different space than the one the
		// resource is currently configured, we won't allow that
		resp.Diagnostics.AddError(
			"Conflict Error",
			fmt.Sprintf(
				"Unable to import integration, the provider is configured in a different space than the resource. (%s != %s)",
				c.Space().ID(), spaceID),
		)
		return nil, false
	}

	return &integration, true
}

func (c *ExtendedGqlClient) ApplyException(
	ctx context.Context,
	scopeMrn string,
	action mondoov1.ExceptionMutationAction,
	checkMrns, controlMrns, cveMrns, vulnerabilityMrns []string,
	justification *string,
	validUntil *string,
	applyToCves *bool,
) error {
	var applyException struct {
		ApplyException bool `graphql:"applyException(input: $input)"`
	}

	// Helper function to convert string slices to *[]mondoov1.String
	convertToGraphQLList := func(mrns []string) *[]mondoov1.String {
		if len(mrns) == 0 {
			return nil
		}
		entries := []mondoov1.String{}
		for _, mrn := range mrns {
			entries = append(entries, mondoov1.String(mrn))
		}
		return &entries
	}

	// Prepare input fields
	input := mondoov1.ExceptionMutationInput{
		ScopeMrn:      mondoov1.String(scopeMrn),
		Action:        action,
		QueryMrns:     convertToGraphQLList(checkMrns),
		ControlMrns:   convertToGraphQLList(controlMrns),
		CveMrns:       convertToGraphQLList(cveMrns),
		AdvisoryMrns:  convertToGraphQLList(vulnerabilityMrns),
		Justification: (*mondoov1.String)(justification),
		ValidUntil:    (*mondoov1.String)(validUntil),
		ApplyToCves:   mondoov1.NewBooleanPtr(mondoov1.Boolean(*applyToCves)),
	}

	return c.Mutate(ctx, &applyException, input, nil)
}
