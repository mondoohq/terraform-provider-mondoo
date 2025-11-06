package mondoovalidator

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func Id() validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-z\d]([\d-_]|[a-z]){2,62}[a-z\d]$`),
		"must contain 4 to 64 digits, dashes, underscores, or lowercase letters, and ending with either a lowercase letter or a digit",
	)
}

func Name() validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(`^([a-zA-Z \-'_]|\d){2,64}$`),
		"must contain 2 to 64 characters, where each character can be a letter (uppercase or lowercase), a space, a dash, an underscore, or a digit",
	)
}
