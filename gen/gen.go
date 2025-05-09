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

// generateIntegrationResources generates Terraform resources for Mondoo's integrations.
func generateIntegrationResources() error {
	funcMap := template.FuncMap{
		"toSnakeCase": toSnakeCase,
		"formatEnum":  formatEnum,
	}

	resourceTemplateFile := filepath.Join("gen", "templates", "integration_resource.go.tmpl")
	resourceTmpl, err := template.New("integration_resource.go.tmpl").
		Funcs(funcMap).
		ParseFiles(resourceTemplateFile)
	if err != nil {
		return err
	}

	testTemplateFile := filepath.Join("gen", "templates", "integration_resource_test.go.tmpl")
	testTmpl, err := template.New("integration_resource_test.go.tmpl").Funcs(funcMap).ParseFiles(testTemplateFile)
	if err != nil {
		return err
	}

	resourceDotTFTemplateFile := filepath.Join("gen", "templates", "resource.tf.tmpl")
	resourceTFTmpl, err := template.New("resource.tf.tmpl").Funcs(funcMap).ParseFiles(resourceDotTFTemplateFile)
	if err != nil {
		return err
	}

	gqlGeneratedTemplateFile := filepath.Join("gen", "templates", "gql_generated.go.tmpl")
	gqlTmpl, err := template.ParseFiles(gqlGeneratedTemplateFile)
	if err != nil {
		return err
	}
	providerGeneratedTmpl, err := template.New("provider_generated.go.tmpl").
		Parse(`// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

var autoGeneratedResources = []func() resource.Resource{
	NewIntegrationAzureResource,
	NewIntegrationAwsResource,
	NewIntegrationAwsServerlessResource,
	NewIntegrationDomainResource,
	NewIntegrationGcpResource,
	NewIntegrationOciTenantResource,
	NewIntegrationSlackResource,
	NewIntegrationMs365Resource,
	NewIntegrationGithubResource,
	NewIntegrationShodanResource,
	NewIntegrationZendeskResource,
	NewIntegrationJiraResource,
	NewIntegrationEmailResource,
	NewIntegrationGitlabResource,
	NewIntegrationMsDefenderResource,
	NewIntegrationCrowdstrikeResource,
	NewIntegrationSentinelOneResource,
	// Auto-generated resources
	{{- range .}}
	NewIntegration{{.}}Resource,
	{{- end}}
}
`)
	if err != nil {
		return err
	}

	importShTmpl, err := template.New("import.sh.tmpl").
		Funcs(funcMap).
		Parse(`# Import using integration MRN.
terraform import mondoo_integration_{{.TerraformResourceName}}.example "//captain.api.mondoo.app/spaces/hungry-poet-123456/integrations/2Abd08lk860"
`)
	if err != nil {
		return err
	}

	// Ensure the output directory exists
	goCodeDirPath := filepath.Join("internal", "provider")
	if err := os.MkdirAll(goCodeDirPath, 0755); err != nil {
		return err
	}
	examplesDirPath := filepath.Join("examples", "resources")
	if err := os.MkdirAll(goCodeDirPath, 0755); err != nil {
		return err
	}

	i := mondoov1.ClientIntegrationConfigurationInput{
		OktaConfigurationOptions:            &mondoov1.OktaConfigurationOptionsInput{},
		GoogleWorkspaceConfigurationOptions: &mondoov1.GoogleWorkspaceConfigurationOptionsInput{},
		// AzureDevopsConfigurationOptions:     &mondoov1.AzureDevopsConfigurationOptionsInput{},
	}
	mapStruct, keys, err := structToMap(i)
	if err != nil {
		return err
	}

	resources := []string{}
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

		// TODO aprse the config options and try to generate a struct that can be passed to the template
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
				// when adding new types, we need to update all the templates
				panic(fmt.Sprintf("unimplemented mondoo api type: %v", t))
			}
		}
		// add the resource class name to the list of resources to use them in the gql_generated.go
		resources = append(resources, className)

		// Create the resource file
		resourceOutputFilePath := filepath.Join(goCodeDirPath,
			fmt.Sprintf("integration_%s_resource.go", terraformResourceName),
		)
		resourceFile, err := os.Create(resourceOutputFilePath)
		if err != nil {
			return err
		}

		var buf bytes.Buffer
		if err := resourceTmpl.Execute(&buf, resource); err != nil {
			return err
		}

		// format go file with gofmt
		out, err := format.Source(buf.Bytes())
		if err != nil {
			log.Println(err)
			out = []byte("// gofmt error: " + err.Error() + "\n\n" + buf.String())
		}

		if _, err := resourceFile.Write(out); err != nil {
			return err
		}
		resourceFile.Close() // done

		// Create test file
		testOutputFilePath := filepath.Join(goCodeDirPath,
			fmt.Sprintf("integration_%s_resource_test.go", terraformResourceName),
		)
		testFile, err := os.Create(testOutputFilePath)
		if err != nil {
			return err
		}

		if err := testTmpl.Execute(testFile, resource); err != nil {
			return err
		}
		testFile.Close() // done

		// Create examples/ files
		resourceExamplesDirPath := filepath.Join(examplesDirPath, fullResourceName)
		// Ensure the output directory exists
		if err := os.MkdirAll(resourceExamplesDirPath, 0755); err != nil {
			return err
		}
		// Create example main.tf
		err = os.WriteFile(filepath.Join(resourceExamplesDirPath, "main.tf"), mainDotTFTestFile(), 0644)
		if err != nil {
			return err
		}
		// Create example resource.tf
		resourceTFOutputFilePath := filepath.Join(resourceExamplesDirPath, "resource.tf")
		resourceTFFile, err := os.Create(resourceTFOutputFilePath)
		if err != nil {
			return err
		}

		if err := resourceTFTmpl.Execute(resourceTFFile, resource); err != nil {
			return err
		}
		resourceTFFile.Close() // done
		// Create example import.sh
		importShOutputFilePath := filepath.Join(resourceExamplesDirPath, "import.sh")
		importShFile, err := os.Create(importShOutputFilePath)
		if err != nil {
			return err
		}

		if err := importShTmpl.Execute(importShFile, resource); err != nil {
			return err
		}
		importShFile.Close() // done
	}

	// Create the gql_generated.go file
	gqlGeneratedOutputFilePath := filepath.Join(goCodeDirPath, "gql_generated.go")
	gqlGeneratedFile, err := os.Create(gqlGeneratedOutputFilePath)
	if err != nil {
		return err
	}
	defer gqlGeneratedFile.Close()

	var buf bytes.Buffer
	if err := gqlTmpl.Execute(&buf, resources); err != nil {
		return err
	}

	// format go file with gofmt
	out, err := format.Source(buf.Bytes())
	if err != nil {
		log.Println(err)
		out = []byte("// gofmt error: " + err.Error() + "\n\n" + buf.String())
	}

	if _, err := gqlGeneratedFile.Write(out); err != nil {
		return err
	}

	// Create the gql_generated.go file
	providerGeneratedOutputFilePath := filepath.Join(goCodeDirPath, "provider_generated.go")
	providerGeneratedFile, err := os.Create(providerGeneratedOutputFilePath)
	if err != nil {
		return err
	}
	defer providerGeneratedFile.Close()

	var pBuf bytes.Buffer
	if err := providerGeneratedTmpl.Execute(&pBuf, resources); err != nil {
		return err
	}

	// format go file with gofmt
	out, err = format.Source(pBuf.Bytes())
	if err != nil {
		log.Println(err)
		out = []byte("// gofmt error: " + err.Error() + "\n\n" + pBuf.String())
	}

	if _, err := providerGeneratedFile.Write(out); err != nil {
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

// @afiune we have to do this whole dance because we do NOT have consistent types.
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

func mainDotTFTestFile() []byte {
	return []byte(`terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.19"
    }
  }
}
`)
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
