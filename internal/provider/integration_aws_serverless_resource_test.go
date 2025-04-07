package provider

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateConfig_HappyPath(t *testing.T) {
	d := &integrationAwsServerlessResourceModel{}
	diagnostics := validateIntegrationAwsServerlessResourceModel(d)
	fmt.Println(diagnostics.Errors())
	assert.False(t, diagnostics.HasError(), "expected no errors")
}
