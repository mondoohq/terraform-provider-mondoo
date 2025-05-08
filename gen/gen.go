// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
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

type Field struct {
	GoType              string
	MondooType          string
	TerraformType       string
	TerraformSchemaType string
	TerraformSubType    string
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

func (f Field) AdditionalSchemaAttributes() string {
	if f.TerraformSubType != "" {
		return fmt.Sprintf("\nElementType: %s,", f.TerraformSubType)
	}
	return ""
}

var (
	BooleanField = Field{
		GoType:              "bool",
		MondooType:          "mondoov1.Boolean",
		TerraformType:       "types.Bool",
		TerraformSchemaType: "schema.BoolAttribute",
	}
	StringField = Field{
		GoType:              "string",
		MondooType:          "mondoov1.String",
		TerraformType:       "types.String",
		TerraformSchemaType: "schema.StringAttribute",
	}
	StringPtrField = Field{
		GoType:              "*string",
		MondooType:          "*mondoov1.String",
		TerraformType:       "types.String",
		TerraformSchemaType: "schema.StringAttribute",
	}
	ArrayStringPtrField = Field{
		GoType:              "[]string",
		MondooType:          "*[]mondoov1.String",
		TerraformType:       "types.List",
		TerraformSubType:    "types.StringType",
		TerraformSchemaType: "schema.ListAttribute",
	}
)

// generateIntegrationResources generates Terraform resources for Mondoo's integrations.
func generateIntegrationResources() error {
	funcMap := template.FuncMap{
		"toSnakeCase": toSnakeCase,
	}

	resourceTemplateFile := filepath.Join("gen", "templates", "integration_resource.go.tmpl")
	resourceTmpl, err := template.New("integration_resource.go.tmpl").
		Funcs(funcMap).
		ParseFiles(resourceTemplateFile)
	if err != nil {
		return err
	}

	testTemplateFile := filepath.Join("gen", "templates", "integration_resource_test.go.tmpl")
	testTmpl, err := template.ParseFiles(testTemplateFile)
	if err != nil {
		return err
	}

	resourceDotTFTemplateFile := filepath.Join("gen", "templates", "resource.tf.tmpl")
	resourceTFTmpl, err := template.ParseFiles(resourceDotTFTemplateFile)
	if err != nil {
		return err
	}

	gqlGeneratedTemplateFile := filepath.Join("gen", "templates", "gql_generated.go.tmpl")
	gqlTmpl, err := template.ParseFiles(gqlGeneratedTemplateFile)
	if err != nil {
		return err
	}

	// Ensure the output directory exists
	outputDirPath := filepath.Join("gen", "generated")
	if err := os.MkdirAll(outputDirPath, 0755); err != nil {
		return err
	}

	i := mondoov1.ClientIntegrationConfigurationInput{
		ShodanConfigurationOptions:          &mondoov1.ShodanConfigurationOptionsInput{},
		OktaConfigurationOptions:            &mondoov1.OktaConfigurationOptionsInput{},
		GoogleWorkspaceConfigurationOptions: &mondoov1.GoogleWorkspaceConfigurationOptionsInput{},
		AzureDevOpsConfigurationOptions:     &mondoov1.AzureDevopsConfigurationOptionsInput{},
	}
	output, err := structToMap(i)
	if err != nil {
		return err
	}

	resources := []string{}
	for k, v := range output {
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
		mm, err := structToMap(v)
		if err != nil {
			log.Fatalf("unable to conver struct %s to map", className)
		}
		if v == nil || len(mm) == 0 {
			fmt.Printf("%s integration has no fields, skipping\n", className)
			continue
		}
		fmt.Printf(">> Generating code for '%s' integration (resource %s)\n", className, fullResourceName)

		// TODO aprse the config options and try to generate a struct that can be passed to the template
		// so that we know the schema of each integration
		for kk, vv := range mm {
			fmt.Println(kk)
			switch t := vv.(type) {
			case mondoov1.Boolean:
				resource.Fields[kk] = BooleanField
			case mondoov1.String:
				resource.Fields[kk] = StringField
			case *mondoov1.String:
				resource.Fields[kk] = StringPtrField
			case *[]mondoov1.String:
				resource.Fields[kk] = ArrayStringPtrField
			default:
				// when adding new types, we need to update all the templates
				panic(fmt.Sprintf("unimplemented mondoo api type: %v", t))
			}
		}
		// add the resource class name to the list of resources to use them in the gql_generated.go
		resources = append(resources, className)

		// Create the resource file
		resourceOutputFilePath := filepath.Join(outputDirPath,
			fmt.Sprintf("integration_%s_resource.go", terraformResourceName),
		)
		resourceFile, err := os.Create(resourceOutputFilePath)
		if err != nil {
			return err
		}
		defer resourceFile.Close()

		if err := resourceTmpl.Execute(resourceFile, resource); err != nil {
			return err
		}

		// Create test file
		testOutputFilePath := filepath.Join(outputDirPath,
			fmt.Sprintf("integration_%s_resource_test.go", terraformResourceName),
		)
		testFile, err := os.Create(testOutputFilePath)
		if err != nil {
			return err
		}
		defer testFile.Close()

		if err := testTmpl.Execute(testFile, resource); err != nil {
			return err
		}

		// Create examples/ files
		examplesDirPath := filepath.Join(outputDirPath, "examples", fullResourceName)
		// Ensure the output directory exists
		if err := os.MkdirAll(examplesDirPath, 0755); err != nil {
			return err
		}
		// Create example main.tf
		err = os.WriteFile(filepath.Join(examplesDirPath, "main.tf"), mainDotTFTestFile(), 0644)
		if err != nil {
			return err
		}
		// Create example resource.tf
		resourceTFOutputFilePath := filepath.Join(examplesDirPath, "resource.tf")
		resourceTFFile, err := os.Create(resourceTFOutputFilePath)
		if err != nil {
			return err
		}
		defer testFile.Close()

		if err := resourceTFTmpl.Execute(resourceTFFile, resource); err != nil {
			return err
		}
	}

	// Create the gql_generated.go file
	gqlGeneratedOutputFilePath := filepath.Join(outputDirPath, "gql_generated.go")
	gqlGeneratedFile, err := os.Create(gqlGeneratedOutputFilePath)
	if err != nil {
		return err
	}
	defer gqlGeneratedFile.Close()

	if err := gqlTmpl.Execute(gqlGeneratedFile, resources); err != nil {
		return err
	}

	return nil
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

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
func structToMap(input interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &output,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}

	err = decoder.Decode(input)
	return output, err
}
