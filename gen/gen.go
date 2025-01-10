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

	"github.com/fatih/structs"
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
}

// generateIntegrationResources generates Terraform resources for Mondoo's integrations.
func generateIntegrationResources() error {
	// Read the template file
	resourceTemplateFile := filepath.Join("gen", "templates", "integration_resource.go.tmpl")
	resourceTmpl, err := template.ParseFiles(resourceTemplateFile)
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

	// Ensure the output directory exists
	outputDirPath := filepath.Join("gen", "generated")
	if err := os.MkdirAll(outputDirPath, 0755); err != nil {
		return err
	}

	i := mondoov1.ClientIntegrationConfigurationInput{}
	m := structs.Map(i)

	for k := range m {
		// TODO we know the type and the struct associated to the type, we need
		// to look it (the struct) and use the same `structs.Map(v)` to list all
		// fields per integration and auto generate the terraform schema and more
		// details, for now, we only leave a comment where we need to add specific
		// integration options

		var (
			className, _          = strings.CutSuffix(k, "ConfigurationOptions")
			terraformResourceName = strings.ToLower(toSnakeCase(className))
			resource              = IntegrationResource{
				ResourceClassName:     className,
				TerraformResourceName: terraformResourceName,
			}
		)

		fmt.Printf("> Generating code for %s integration\n", className)

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
		examplesDirPath := filepath.Join(outputDirPath, "examples", fmt.Sprintf("mondoo_integration_%s", terraformResourceName))
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
