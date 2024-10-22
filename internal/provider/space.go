// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import "strings"

const spacePrefix = "//captain.api.mondoo.app/spaces/"

// Helper type to handle both, space id and space mrn, interchangeably.
type Space string

// SpaceFrom receives either a space id or a space mrn and returns a `Space`.
func SpaceFrom(space string) Space {
	if strings.HasPrefix(space, spacePrefix) {
		// MRN
		spaceID := strings.Split(space, "/")[4]
		// Using this index is safe since we check for a prefix that has 4 slashes
		return Space(spaceID)
	}
	// ID
	return Space(space)
}

func (s Space) ID() string {
	return string(s)
}
func (s Space) MRN() string {
	if s == "" {
		return ""
	}
	return spacePrefix + string(s)
}
