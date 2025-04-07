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

	i := mondoov1.ClientIntegrationConfigurationInput{
		ShodanConfigurationOptions: &mondoov1.ShodanConfigurationOptionsInput{},
	}
	output, err := structToMap(i)
	if err != nil {
		return err
	}

	for k, v := range output {
		var (
			className, _          = strings.CutSuffix(k, "ConfigurationOptions")
			terraformResourceName = strings.ToLower(toSnakeCase(className))
			fullResourceName      = fmt.Sprintf("mondoo_integration_%s", terraformResourceName)
			resource              = IntegrationResource{
				ResourceClassName:     className,
				TerraformResourceName: terraformResourceName,
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
		fmt.Printf("generating code for '%s' integration (resource %s)\n", className, fullResourceName)

		// TODO aprse the config options and try to generate a struct that can be passed to the template
		// so that we know the schema of each integration
		for kk, vv := range mm {
			fmt.Println(kk)
			switch vv.(type) {
			case mondoov1.String:
				fmt.Println("string")
			case *mondoov1.String:
				fmt.Println("stringptr")
			case *[]mondoov1.String:
				fmt.Println("stringptr array")
			}
		}

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
