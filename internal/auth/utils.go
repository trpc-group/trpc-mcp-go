// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package auth

import (
	"fmt"
	"net/url"
	"strings"
)

// Utilities for handling OAuth resource URIs.

// ResourceURLFromServerURL converts a server URL to a resource URL by removing the fragment.
func ResourceURLFromServerURL(u interface{}) (*url.URL, error) {
	var resourceURL *url.URL
	var err error

	switch v := u.(type) {
	case string:
		resourceURL, err = url.Parse(v)
		if err != nil {
			return nil, err
		}
	case *url.URL:
		resourceURL, err = url.Parse(v.String())
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported URL type")
	}

	// Remove fragment
	resourceURL.Fragment = ""
	return resourceURL, nil
}

// CheckResourceAllowedParams represents the parameters for CheckResourceAllowed function
type CheckResourceAllowedParams struct {
	RequestedResource  interface{} // URL string or *url.URL
	ConfiguredResource interface{} // URL string or *url.URL
}

// CheckResourceAllowed checks if a requested resource URL matches a configured resource URL.
func CheckResourceAllowed(params CheckResourceAllowedParams) (bool, error) {
	requested, err := parseURL(params.RequestedResource)
	if err != nil {
		return false, err
	}

	configured, err := parseURL(params.ConfiguredResource)
	if err != nil {
		return false, err
	}

	// Compare the origin (scheme, domain, and port)
	if requested.Scheme != configured.Scheme ||
		requested.Host != configured.Host {
		return false, nil
	}

	// Handle cases like requested=/foo and configured=/foo/
	if len(requested.Path) < len(configured.Path) {
		return false, nil
	}

	requestedPath := requested.Path
	if !strings.HasSuffix(requestedPath, "/") {
		requestedPath = requestedPath + "/"
	}

	configuredPath := configured.Path
	if !strings.HasSuffix(configuredPath, "/") {
		configuredPath = configuredPath + "/"
	}

	return strings.HasPrefix(requestedPath, configuredPath), nil
}

// parseURL is a helper function to parse URL from string or *url.URL
func parseURL(u interface{}) (*url.URL, error) {
	switch v := u.(type) {
	case string:
		return url.Parse(v)
	case *url.URL:
		return url.Parse(v.String())
	default:
		return nil, fmt.Errorf("unsupported URL type")
	}
}
