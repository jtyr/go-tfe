package tfe

import (
	"context"
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
	// List all the providers within an organization.
	List(ctx context.Context, organization string, options *RegistryProviderListOptions) (*RegistryProviderList, error)

	// Create a registry provider
	Create(ctx context.Context, organization string, options RegistryProviderCreateOptions) (*RegistryProvider, error)

	// Read a registry provider
	Read(ctx context.Context, providerId RegistryProviderID, options *RegistryProviderReadOptions) (*RegistryProvider, error)

	// Delete a registry provider
	Delete(ctx context.Context, providerId RegistryProviderID) error
}

// registryProviders implements RegistryProviders.
type registryProviders struct {
	client *Client
}

// RegistryName represents which registry is being targeted
type RegistryName string

// List of available registry names
const (
	PrivateRegistry RegistryName = "private"
	PublicRegistry  RegistryName = "public"
)

func (rn RegistryName) valid() error {
	switch rn {
	case PrivateRegistry, PublicRegistry:
		return nil
	}
	return ErrInvalidRegistryName
}

// RegistryProviderIncludeOps represents which jsonapi include can be used with registry providers
type RegistryProviderIncludeOps string

// List of available includes
const (
	RegistryProviderVersionsInclude RegistryProviderIncludeOps = "registry-provider-versions"
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
	Organization             *Organization              `jsonapi:"relation,organization"`
	RegistryProviderVersions []*RegistryProviderVersion `jsonapi:"relation,registry-provider-versions"`
}

type RegistryProviderPermissions struct {
	CanDelete bool `jsonapi:"attr,can-delete"`
}

type RegistryProviderListOptions struct {
	ListOptions
	// A query string to filter by registry_name
	RegistryName RegistryName `url:"filter[registry_name],omitempty"`
	// A query string to filter by organization
	OrganizationName string `url:"filter[organization_name],omitempty"`
	// A query string to do a fuzzy search
	Search string `url:"q,omitempty"`

	// Include related jsonapi relationships
	Include *[]RegistryProviderIncludeOps `url:"include,omitempty"`
}

type RegistryProviderList struct {
	*Pagination
	Items []*RegistryProvider
}

func (o RegistryProviderListOptions) valid() error {
	return nil
}

func (r *registryProviders) List(ctx context.Context, organization string, options *RegistryProviderListOptions) (*RegistryProviderList, error) {

	if !validStringID(&organization) {
		return nil, ErrInvalidOrg
	}
	if options != nil {
		if err := options.valid(); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf("organizations/%s/registry-providers", url.QueryEscape(organization))
	req, err := r.client.newRequest("GET", u, options)
	if err != nil {
		return nil, err
	}

	pl := &RegistryProviderList{}
	err = r.client.do(ctx, req, pl)
	if err != nil {
		return nil, err
	}

	return pl, nil
}

// RegistryProviderCreateOptions is used when creating a registry provider
type RegistryProviderCreateOptions struct {
	// Type is a public field utilized by JSON:API to
	// set the resource type via the field tag.
	// It is not a user-defined value and does not need to be set.
	// https://jsonapi.org/format/#crud-creating
	Type string `jsonapi:"primary,registry-providers"`

	Namespace    string       `jsonapi:"attr,namespace"`
	Name         string       `jsonapi:"attr,name"`
	RegistryName RegistryName `jsonapi:"attr,registry-name"`
}

func (o RegistryProviderCreateOptions) valid() error {
	if !validStringID(&o.Name) {
		return ErrInvalidName
	}
	if !validStringID(&o.Namespace) {
		return ErrInvalidNamespace
	}
	if err := o.RegistryName.valid(); err != nil {
		return err
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
	if options.RegistryName == PrivateRegistry && organization != options.Namespace {
		return nil, ErrInvalidPrivateProviderNamespaceDoesntMatchOrganization
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

// RegistryProviderID is the multi key ID for addressing a provider
type RegistryProviderID struct {
	OrganizationName string       `jsonapi:"attr,organization-name"`
	Namespace        string       `jsonapi:"attr,namespace"`
	Name             string       `jsonapi:"attr,name"`
	RegistryName     RegistryName `jsonapi:"attr,registry-name"`
}

func (id RegistryProviderID) valid() error {
	if !validStringID(&id.OrganizationName) {
		return ErrInvalidOrg
	}
	if !validStringID(&id.Name) {
		return ErrInvalidName
	}
	if !validStringID(&id.Namespace) {
		return ErrInvalidNamespace
	}
	if err := id.RegistryName.valid(); err != nil {
		return err
	}
	return nil
}

type RegistryProviderReadOptions struct {
	// Include related jsonapi relationships
	Include *[]RegistryProviderIncludeOps `url:"include,omitempty"`
}

func (r *registryProviders) Read(ctx context.Context, providerId RegistryProviderID, options *RegistryProviderReadOptions) (*RegistryProvider, error) {
	if err := providerId.valid(); err != nil {
		return nil, err
	}

	u := fmt.Sprintf(
		"organizations/%s/registry-providers/%s/%s/%s",
		url.QueryEscape(providerId.OrganizationName),
		url.QueryEscape(string(providerId.RegistryName)),
		url.QueryEscape(providerId.Namespace),
		url.QueryEscape(providerId.Name),
	)
	req, err := r.client.newRequest("GET", u, options)
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

func (r *registryProviders) Delete(ctx context.Context, providerId RegistryProviderID) error {
	if err := providerId.valid(); err != nil {
		return err
	}

	u := fmt.Sprintf(
		"organizations/%s/registry-providers/%s/%s/%s",
		url.QueryEscape(providerId.OrganizationName),
		url.QueryEscape(string(providerId.RegistryName)),
		url.QueryEscape(providerId.Namespace),
		url.QueryEscape(providerId.Name),
	)
	req, err := r.client.newRequest("DELETE", u, nil)
	if err != nil {
		return err
	}

	return r.client.do(ctx, req, nil)
}
