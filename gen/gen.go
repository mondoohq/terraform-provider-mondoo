// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/go-viper/mapstructure/v2"
	mondoov1 "go.mondoo.com/mondoo-go"
)

func main() {
	// Generate resources for Mondoo integrations only
	err := generateIntegrationResources()
	if err != nil {
		log.Fatalln(err)
	}
}

type IntegrationResource struct {
	ResourceClassName     string
	TerraformResourceName string
	Fields                map[string]Field
}

func NewField(base Field, raw any) Field {
	base.RawStruct = raw
	return base
}

type Field struct {
	RawStruct           any
	GoType              string
	MondooType          string
	TerraformType       string
	TerraformSchemaType string
	TerraformSubType    string
	HclType             string
	GoFmtVerb           string
}

func (f Field) GoTestValue(name string, testcase int) string {
	switch f.MondooType {
	case BooleanField.MondooType:
		if testcase%2 == 0 {
			return "true"
		}
		return "false"
	case StringField.MondooType:
		return fmt.Sprintf("\"%s_%d\"", name, testcase)
	case StringPtrField.MondooType:
		return fmt.Sprintf("\"%s_%d\"", name, testcase)
	case ArrayStringPtrField.MondooType:
		return fmt.Sprintf("[]string{\"%s_%d\"}", name, testcase)
	}
	return "\"unimplemented: check gen/gen.go\""
}

func (f Field) ConfigurationOption(name string) string {
	switch f.MondooType {
	case BooleanField.MondooType:
		return fmt.Sprintf("mondoov1.Boolean(m.%s.ValueBool())", name)
	case StringField.MondooType:
		return fmt.Sprintf("mondoov1.String(m.%s.ValueString())", name)
	case StringPtrField.MondooType:
		return fmt.Sprintf("mondoov1.NewStringPtr(mondoov1.String(m.%s.ValueString()))", name)
	case ArrayStringPtrField.MondooType:
		return fmt.Sprintf("ToPtr(ConvertSliceStrings(m.%s))", name)
	}
	return "\"unimplemented: check gen/gen.go\""
}

func (f Field) ImportConvertion(resourceClassName, fieldName string) string {
	attr := fmt.Sprintf("integration.ConfigurationOptions.%sConfigurationOptions.%s", resourceClassName, fieldName)
	switch f.MondooType {
	case BooleanField.MondooType:
		return fmt.Sprintf("types.BoolValue(%s)", attr)
	case StringField.MondooType:
		return fmt.Sprintf("types.StringValue(%s)", attr)
	case StringPtrField.MondooType:
		return fmt.Sprintf("types.StringPointerValue(%s)", attr)
	case ArrayStringPtrField.MondooType:
		return fmt.Sprintf("ConvertListValue(%s)", attr)
	}
	return "\"unimplemented: check gen/gen.go\""
}

func (f Field) AttributeOptionalOrRequired(name string) string {
	rawField, ok := findField(f.RawStruct, name)
	if !ok {
		panic("field in struct not found")
	}

	tag := parseTFGenTag(rawField.Tag.Get("tfgen"))
	if tag.Required {
		return "Required: true"
	}
	return "Optional: true"
}

type tfgenTag struct {
	Required    bool
	Description string
}

func parseTFGenTag(tag string) tfgenTag {
	t := tfgenTag{}
	for _, val := range strings.Split(tag, ";") {
		switch {
		case strings.HasPrefix(val, "required=1"):
			t.Required = true
		case strings.HasPrefix(val, "description="):
			t.Description, _ = strings.CutPrefix(val, "description=")
		}
	}
	return t
}

func (f Field) AdditionalSchemaAttributes() string {
	if f.TerraformSubType != "" {
		return fmt.Sprintf("\nElementType: %s,", f.TerraformSubType)
	}
	return ""
}

func findField(obj any, fieldName string) (reflect.StructField, bool) {
	val := reflect.TypeOf(obj)

	// If the object is a pointer, get the underlying element
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Make sure the object is a struct
	if val.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}

	return val.FieldByName(fieldName)
}

var (
	BooleanField = Field{
		GoType:              "bool",
		MondooType:          "mondoov1.Boolean",
		TerraformType:       "types.Bool",
		TerraformSchemaType: "schema.BoolAttribute",
		HclType:             "bool",
		GoFmtVerb:           "t",
	}
	StringField = Field{
		GoType:              "string",
		MondooType:          "mondoov1.String",
		TerraformType:       "types.String",
		TerraformSchemaType: "schema.StringAttribute",
		HclType:             "string",
		GoFmtVerb:           "q",
	}
	StringPtrField = Field{
		GoType:              "*string",
		MondooType:          "*mondoov1.String",
		TerraformType:       "types.String",
		TerraformSchemaType: "schema.StringAttribute",
		HclType:             "string",
		GoFmtVerb:           "q",
	}
	ArrayStringPtrField = Field{
		GoType:              "[]string",
		MondooType:          "*[]mondoov1.String",
		TerraformType:       "types.List",
		TerraformSubType:    "types.StringType",
		TerraformSchemaType: "schema.ListAttribute",
		HclType:             "list(string)",
		GoFmtVerb:           "q",
	}
)

var funcMap = template.FuncMap{
	"toSnakeCase": toSnakeCase,
	"formatEnum":  formatEnum,
}

var templates = map[string]*template.Template{
	"integration_resource.go": template.Must(template.New("integration_resource.go.tmpl").Funcs(funcMap).
		ParseFiles(filepath.Join("gen", "templates", "integration_resource.go.tmpl"))),
	"integration_resource_test.go": template.Must(template.New("integration_resource_test.go.tmpl").Funcs(funcMap).
		ParseFiles(filepath.Join("gen", "templates", "integration_resource_test.go.tmpl"))),
	"resource.tf": template.Must(template.New("resource.tf.tmpl").Funcs(funcMap).
		ParseFiles(filepath.Join("gen", "templates", "resource.tf.tmpl"))),
	"gql_generated.go": template.Must(template.New("gql_generated.go.tmpl").Funcs(funcMap).
		ParseFiles(filepath.Join("gen", "templates", "gql_generated.go.tmpl"))),
	"provider_generated.go": template.Must(template.New("").Funcs(funcMap).
		Parse(`// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

var autoGeneratedResources = []func() resource.Resource{
	NewIntegrationAwsResource,
	NewIntegrationAwsServerlessResource,
	NewIntegrationAzureResource,
	NewIntegrationCrowdstrikeResource,
	NewIntegrationDomainResource,
	NewIntegrationEmailResource,
	NewIntegrationGcpResource,
	NewIntegrationGithubResource,
	NewIntegrationGitlabResource,
	NewIntegrationJiraResource,
	NewIntegrationMsDefenderResource,
	NewIntegrationMs365Resource,
	NewIntegrationOciTenantResource,
	NewIntegrationSentinelOneResource,
	NewIntegrationShodanResource,
	NewIntegrationSlackResource,
	NewIntegrationZendeskResource,
	// Auto-generated resources
	{{- range .}}
	NewIntegration{{.}}Resource,
	{{- end}}
}
`)),
	"import.sh": template.Must(template.New("").Funcs(funcMap).
		Parse(`# Import using integration MRN.
terraform import mondoo_integration_{{.TerraformResourceName}}.example "//captain.api.mondoo.app/spaces/hungry-poet-123456/integrations/2Abd08lk860"
`)),
	"main.tf": template.Must(template.New("").Funcs(funcMap).
		Parse(`terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.19"
    }
  }
}
`)),
}

func renderTemplate(filePath string, tmpl *template.Template, data any) error {
	resourceFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer resourceFile.Close() // done

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	var out []byte
	if strings.HasSuffix(filePath, ".go") {
		// format go file with gofmt
		out, err = format.Source(buf.Bytes())
		if err != nil {
			return err
			// log.Println(err)
			// out = []byte("// gofmt error: " + err.Error() + "\n\n" + buf.String())
		}
	} else if strings.HasSuffix(filePath, ".tf") {
		// TODO format tf file with 'terraform fmt'
		out = buf.Bytes()
	} else {
		out = buf.Bytes()
	}

	_, err = resourceFile.Write(out)
	return err
}

// generateIntegrationResources generates Terraform resources for Mondoo's integrations.
func generateIntegrationResources() error {
	// Ensure the output directory exists
	goCodeDirPath := filepath.Join("internal", "provider")
	if err := os.MkdirAll(goCodeDirPath, 0755); err != nil {
		return err
	}
	examplesDirPath := filepath.Join("examples", "resources")
	if err := os.MkdirAll(goCodeDirPath, 0755); err != nil {
		return err
	}

	// TODO: Flip this around, instead of adding new structs to auto-generate resources, have them all turned on and
	// disable the ones we already generated (why? To avoid breaking changes)
	i := mondoov1.ClientIntegrationConfigurationInput{
		OktaConfigurationOptions:            &mondoov1.OktaConfigurationOptionsInput{},
		GoogleWorkspaceConfigurationOptions: &mondoov1.GoogleWorkspaceConfigurationOptionsInput{},
		// AzureDevopsConfigurationOptions:     &mondoov1.AzureDevopsConfigurationOptionsInput{},
	}
	mapStruct, keys, err := structToMap(i)
	if err != nil {
		return err
	}

	// Store the list of generated Terraform resources so that at the end of the loop we auto-generate
	// other files that depend on the list of resources, like the `provider_generated.go`
	resources := []string{}

	// Iterate over the keys to generate resources in order
	for _, k := range keys {
		v := mapStruct[k]
		var (
			className, _          = strings.CutSuffix(k, "ConfigurationOptions")
			terraformResourceName = strings.ToLower(toSnakeCase(className))
			fullResourceName      = fmt.Sprintf("mondoo_integration_%s", terraformResourceName)
			resource              = IntegrationResource{
				ResourceClassName:     className,
				TerraformResourceName: terraformResourceName,
				Fields:                map[string]Field{},
			}
		)
		mm, mmKeys, err := structToMap(v)
		if err != nil {
			log.Fatalf("unable to conver struct %s to map", className)
		}
		if v == nil || len(mm) == 0 {
			fmt.Printf("❌ %s integration has no fields, skipping\n", className)
			continue
		}
		fmt.Printf(">>> ⭐ Generating code for '%s' integration (resource %s)\n", className, fullResourceName)

		// TODO parse the config options and try to generate a struct that can be passed to the template
		// so that we know the schema of each integration
		for _, kk := range mmKeys {
			vv := mm[kk]
			switch t := vv.(type) {
			case mondoov1.Boolean:
				resource.Fields[kk] = NewField(BooleanField, v)
			case mondoov1.String:
				resource.Fields[kk] = NewField(StringField, v)
			case *mondoov1.String:
				resource.Fields[kk] = NewField(StringPtrField, v)
			case *[]mondoov1.String:
				resource.Fields[kk] = NewField(ArrayStringPtrField, v)
			default:
				// when adding new types, we might need to update all the templates
				panic(fmt.Sprintf("unimplemented mondoo api type: %v", t))
			}
		}
		// add the resource class name to the list of resources to use them in the gql_generated.go
		resources = append(resources, className)

		// Create the resource file
		err = renderTemplate(
			filepath.Join(goCodeDirPath, fmt.Sprintf("integration_%s_resource.go", terraformResourceName)),
			templates["integration_resource.go"],
			resource,
		)
		if err != nil {
			return err
		}

		// Create test file
		err = renderTemplate(
			filepath.Join(goCodeDirPath, fmt.Sprintf("integration_%s_resource_test.go", terraformResourceName)),
			templates["integration_resource_test.go"],
			resource,
		)
		if err != nil {
			return err
		}

		// Create examples/ files
		resourceExamplesDirPath := filepath.Join(examplesDirPath, fullResourceName)
		// Ensure the output directory exists
		if err := os.MkdirAll(resourceExamplesDirPath, 0755); err != nil {
			return err
		}
		// Create example main.tf
		err = renderTemplate(filepath.Join(resourceExamplesDirPath, "main.tf"), templates["main.tf"], resource)
		if err != nil {
			return err
		}
		// Create example resource.tf
		err = renderTemplate(filepath.Join(resourceExamplesDirPath, "resource.tf"), templates["resource.tf"], resource)
		if err != nil {
			return err
		}
		// Create example import.sh
		err = renderTemplate(filepath.Join(resourceExamplesDirPath, "import.sh"), templates["import.sh"], resource)
		if err != nil {
			return err
		}
	}

	// Create the gql_generated.go file
	err = renderTemplate(filepath.Join(goCodeDirPath, "gql_generated.go"), templates["gql_generated.go"], resources)
	if err != nil {
		return err
	}
	err = renderTemplate(filepath.Join(goCodeDirPath, "provider_generated.go"), templates["provider_generated.go"], resources)
	if err != nil {
		return err
	}

	return nil
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

var ticketSystemIntegrations = []string{
	"Jira",
	"Email",
	"Zendesk",
	"Github",
	"Gitlab",
	"AzureDevops",
}

// @afiune we have to do this whole dance because we do not have consistent types.
func formatEnum(enum string) string {
	if isTicketIntegration(enum) {
		return "TicketSystem" + enum
	}
	return enum
}

func isTicketIntegration(enum string) bool {
	for _, iType := range ticketSystemIntegrations {
		if strings.HasSuffix(strings.ToLower(enum), strings.ToLower(iType)) {
			return true
		}
	}
	return false
}
func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func structToMap(input any) (map[string]any, []string, error) {
	output := make(map[string]any)
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &output,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, nil, err
	}

	err = decoder.Decode(input)
	if err != nil {
		return output, nil, err
	}

	keys := make([]string, 0, len(output))
	for k := range output {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return output, keys, err
}
