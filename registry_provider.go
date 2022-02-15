package tfe

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

// Compile-time proof of interface implementation.
var _ RegistryProviders = (*registryProviders)(nil)

// RegistryProviders describes all the registry provider related methods that the Terraform
// Enterprise API supports.
//
// TFE API docs: https://www.terraform.io/docs/cloud/api/providers.html
type RegistryProviders interface {
	// Create a registry provider
	Create(ctx context.Context, organization string, options RegistryProviderCreateOptions) (*RegistryProvider, error)
}

// registryProviders implements RegistryProviders.
type registryProviders struct {
	client *Client
}

// RegistryName represents which registry is being targeted
type RegistryName string

// List of available registry names
const (
	Private RegistryName = "private"
	Public  RegistryName = "public"
)

// RegistryProvider represents a registry provider
type RegistryProvider struct {
	ID           string                       `jsonapi:"primary,registry-providers"`
	Namespace    string                       `jsonapi:"attr,namespace"`
	Name         string                       `jsonapi:"attr,name"`
	RegistryName RegistryName                 `jsonapi:"attr,registry-name"`
	Permissions  *RegistryProviderPermissions `jsonapi:"attr,permissions"`
	CreatedAt    string                       `jsonapi:"attr,created-at"`
	UpdatedAt    string                       `jsonapi:"attr,updated-at"`

	// Relations
	Organization *Organization `jsonapi:"relation,organization"`
}

type RegistryProviderPermissions struct {
	CanDelete bool `jsonapi:"attr,can-delete"`
}

// RegistryProviderVersion represents a registry provider version
type RegistryProviderVersion struct {
	ID        string   `jsonapi:"primary,registry-provider-versions"`
	Version   string   `jsonapi:"attr,version"`
	KeyID     string   `jsonapi:"attr,key-id"`
	Protocols []string `jsonapi:"attr,protocols,omitempty"`

	// Relations
	RegistryProvider *RegistryProvider `jsonapi:"relation,registry-provider"`

	// Links
	Links map[string]interface{} `jsonapi:"links,omitempty"`
}

// RegistryProviderPlatform represents a registry provider platform
type RegistryProviderPlatform struct {
	ID       string `jsonapi:"primary,registry-provider-platforms"`
	Os       string `jsonapi:"attr,os"`
	Arch     string `jsonapi:"attr,arch"`
	Filename string `jsonapi:"attr,filename"`
	SHASUM   string `jsonapi:"attr,shasum"`

	// Relations
	RegistryProviderVersion *RegistryProviderVersion `jsonapi:"relation,registry-provider-version"`

	// Links
	Links map[string]interface{} `jsonapi:"links,omitempty"`
}

// RegistryProviderCreateOptions is used when creating a registry provider
type RegistryProviderCreateOptions struct {
	// Type is a public field utilized by JSON:API to
	// set the resource type via the field tag.
	// It is not a user-defined value and does not need to be set.
	// https://jsonapi.org/format/#crud-creating
	Type string `jsonapi:"primary,registry-providers"`

	Namespace    *string       `jsonapi:"attr,namespace"`
	Name         *string       `jsonapi:"attr,name"`
	RegistryName *RegistryName `jsonapi:"attr,registry-name"`
}

func (o RegistryProviderCreateOptions) valid() error {
	if !validString(o.Name) {
		return ErrRequiredName
	}
	if !validStringID(o.Name) {
		return ErrInvalidName
	}
	if !validString(o.Namespace) {
		return errors.New("namespace is required")
	}
	if !validStringID(o.Namespace) {
		return errors.New("invalid value for namespace")
	}
	if o.RegistryName == nil {
		return errors.New("registry-name is required")
	}
	return nil
}

func (r *registryProviders) Create(ctx context.Context, organization string, options RegistryProviderCreateOptions) (*RegistryProvider, error) {
	if !validStringID(&organization) {
		return nil, ErrInvalidOrg
	}
	if err := options.valid(); err != nil {
		return nil, err
	}
	// Private providers must match their namespace and organization name
	// This is enforced by the API as well
	if *options.RegistryName == Private && organization != *options.Namespace {
		return nil, errors.New("namespace must match organization name for private providers")
	}

	u := fmt.Sprintf(
		"organizations/%s/registry-providers",
		url.QueryEscape(organization),
	)
	req, err := r.client.newRequest("POST", u, &options)
	if err != nil {
		return nil, err
	}
	prv := &RegistryProvider{}
	err = r.client.do(ctx, req, prv)
	if err != nil {
		return nil, err
	}

	return prv, nil
}
