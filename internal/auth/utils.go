package auth

import (
	"fmt"
	"net/url"
	"strings"
)

// Utilities for handling OAuth resource URIs.

// ResourceURLFromServerURL converts a server URL to a resource URL by removing the fragment.
// RFC 8707 section 2 states that resource URIs "MUST NOT include a fragment component".
// Keeps everything else unchanged (scheme, domain, port, path, query).
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
// A requested resource matches if it has the same scheme, domain, port,
// and its path starts with the configured resource's path.
//
// Parameters:
//   - params.RequestedResource: The resource URL being requested
//   - params.ConfiguredResource: The resource URL that has been configured
//
// Returns true if the requested resource matches the configured resource, false otherwise
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

	// Check if the requested path starts with the configured path
	// Ensure both paths end with / for proper comparison
	// This ensures that if we have paths like "/api" and "/api/users",
	// we properly detect that "/api/users" is a subpath of "/api"
	// By adding a trailing slash if missing, we avoid false positives
	// where paths like "/api123" would incorrectly match "/api"
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
