package tfe

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

// Compile-time proof of interface implementation.
//var _ RegistryProviderVersions = (*registryProviderVersions)(nil)

// RegistryProviders describes all the registry provider related methods that the Terraform
// Enterprise API supports.
//
// TFE API docs: https://www.terraform.io/docs/cloud/api/providers.html
type RegistryProviderVersions interface {
	// List all the providers within an organization.
	List(ctx context.Context, providerId RegistryProviderID, options *RegistryProviderVersionListOptions) (*RegistryProviderVersionList, error)

	// Create a registry provider
	Create(ctx context.Context, providerId RegistryProviderID, options RegistryProviderVersionCreateOptions) (*RegistryProviderVersion, error)

	// Read a registry provider
	Read(ctx context.Context, versionId RegistryProviderVersionID, options *RegistryProviderVersionReadOptions) (*RegistryProvider, error)

	// Delete a registry provider
	Delete(ctx context.Context, versionId RegistryProviderVersionID) error
}

// registryProviders implements RegistryProviders.
type registryProviderVersions struct {
	client *Client
}

// RegistryProviderVersion represents a registry provider version
type RegistryProviderVersion struct {
	ID        string   `jsonapi:"primary,registry-provider-versions"`
	Version   string   `jsonapi:"attr,version"`
	KeyID     string   `jsonapi:"attr,key-id"`
	Protocols []string `jsonapi:"attr,protocols,omitempty"`
	CreatedAt string   `jsonapi:"attr,created-at"`
	UpdatedAt string   `jsonapi:"attr,updated-at"`

	// Relations
	RegistryProvider          *RegistryProvider          `jsonapi:"relation,registry-provider"`
	RegistryProviderPlatforms []RegistryProviderPlatform `jsonapi:"relation,registry-provider-platform"`

	// Links
	Links map[string]interface{} `jsonapi:"links,omitempty"`
}

// RegistryProviderID is the multi key ID for addressing a provider
type RegistryProviderVersionID struct {
	RegistryProviderID
	Version string `jsonapi:"attr,version"`
}

func (id RegistryProviderVersionID) valid() error {
	if !validStringID(&id.Version) {
		return errors.New("version is required")
	}
	if id.RegistryName != PrivateRegistry {
		return errors.New("only private registry is allowed")
	}
	if err := id.RegistryProviderID.valid(); err != nil {
		return err
	}
	return nil
}

type RegistryProviderVersionList struct {
	*Pagination
	Items []*RegistryProvider
}

type RegistryProviderVersionListOptions struct {
	ListOptions
}

func (o RegistryProviderVersionListOptions) valid() error {
	return nil
}

func (r *registryProviderVersions) List(ctx context.Context, providerId RegistryProviderID, options *RegistryProviderVersionListOptions) (*RegistryProviderVersionList, error) {
	if err := providerId.valid(); err != nil {
		return nil, err
	}
	if options != nil {
		if err := options.valid(); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf(
		"organizations/%s/registry-providers/%s/%s/%s/versions",
		url.QueryEscape(providerId.OrganizationName),
		url.QueryEscape(string(providerId.RegistryName)),
		url.QueryEscape(providerId.Namespace),
		url.QueryEscape(providerId.Name),
	)
	req, err := r.client.newRequest("GET", u, options)
	if err != nil {
		return nil, err
	}

	pvl := &RegistryProviderVersionList{}
	err = r.client.do(ctx, req, pvl)
	if err != nil {
		return nil, err
	}

	return pvl, nil
}

type RegistryProviderVersionCreateOptions struct {
	Version string `jsonapi:"attr,version"`
	KeyID   string `jsonapi:"attr,key-id"`
}

func (o RegistryProviderVersionCreateOptions) valid() error {
	if !validStringID(&o.Version) {
		return errors.New("version is required")
	}
	if !validStringID(&o.KeyID) {
		return errors.New("key-id is required")
	}
	return nil
}

// Create a registry provider
func (r *registryProviderVersions) Create(ctx context.Context, providerId RegistryProviderID, options RegistryProviderVersionCreateOptions) (*RegistryProviderVersion, error) {
	if err := providerId.valid(); err != nil {
		return nil, err
	}
	if providerId.RegistryName != PrivateRegistry {
		return nil, errors.New("only private registry is allowed")
	}
	if err := options.valid(); err != nil {
		return nil, err
	}

	u := fmt.Sprintf(
		"organizations/%s/registry-providers/%s/%s/%s/versions",
		url.QueryEscape(providerId.OrganizationName),
		url.QueryEscape(string(providerId.RegistryName)),
		url.QueryEscape(providerId.Namespace),
		url.QueryEscape(providerId.Name),
	)
	req, err := r.client.newRequest("POST", u, &options)
	if err != nil {
		return nil, err
	}
	prvv := &RegistryProviderVersion{}
	err = r.client.do(ctx, req, prvv)
	if err != nil {
		return nil, err
	}

	return prvv, nil
}

type RegistryProviderVersionReadOptions struct{}

// Read a registry provider
func (r *registryProviderVersions) Read(ctx context.Context, versionId RegistryProviderVersionID, options *RegistryProviderVersionReadOptions) (*RegistryProvider, error) {
	return nil, nil
}

// Delete a registry provider
func (r *registryProviderVersions) Delete(ctx context.Context, versionId RegistryProviderVersionID) error {
	return nil
}
