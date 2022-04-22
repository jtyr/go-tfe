package tfe

import (
	"context"
	"fmt"
	"net/url"
)

// Compile-time proof of interface implementation.
var _ RegistryProviderVersions = (*registryProviderVersions)(nil)

// RegistryProviderVersions describes all the registry provider related methods that the Terraform
// Enterprise API supports.
//
// TFE API docs: https://www.terraform.io/docs/cloud/api/providers.html
type RegistryProviderVersions interface {
	// List all the providers within an organization.
	List(ctx context.Context, providerID RegistryProviderID, options *RegistryProviderVersionListOptions) (*RegistryProviderVersionList, error)

	// Create a registry provider
	Create(ctx context.Context, providerID RegistryProviderID, options RegistryProviderVersionCreateOptions) (*RegistryProviderVersion, error)

	// Read a registry provider
	Read(ctx context.Context, versionID RegistryProviderVersionID, options *RegistryProviderVersionReadOptions) (*RegistryProviderVersion, error)

	// Delete a registry provider
	Delete(ctx context.Context, versionID RegistryProviderVersionID) error
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
	RegistryProvider          *RegistryProvider           `jsonapi:"relation,registry-provider"`
	RegistryProviderPlatforms []*RegistryProviderPlatform `jsonapi:"relation,registry-provider-platforms"`

	// Links
	Links map[string]interface{} `jsonapi:"links,omitempty"`
}

// RegistryProviderID is the multi key ID for addressing a provider
type RegistryProviderVersionID struct {
	RegistryProviderID
	Version string `jsonapi:"attr,version"`
}

type RegistryProviderVersionList struct {
	*Pagination
	Items []*RegistryProviderVersion
}

type RegistryProviderVersionListOptions struct {
	ListOptions
}

type RegistryProviderVersionReadOptions struct{}

type RegistryProviderVersionCreateOptions struct {
	Version string `jsonapi:"attr,version"`
	KeyID   string `jsonapi:"attr,key-id"`
}

func (r *registryProviderVersions) List(ctx context.Context, providerID RegistryProviderID, options *RegistryProviderVersionListOptions) (*RegistryProviderVersionList, error) {
	if err := providerID.valid(); err != nil {
		return nil, err
	}
	if options != nil {
		if err := options.valid(); err != nil {
			return nil, err
		}
	}

	u := fmt.Sprintf(
		"organizations/%s/registry-providers/%s/%s/%s/versions",
		url.QueryEscape(providerID.OrganizationName),
		url.QueryEscape(string(providerID.RegistryName)),
		url.QueryEscape(providerID.Namespace),
		url.QueryEscape(providerID.Name),
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

// Create a registry provider
func (r *registryProviderVersions) Create(ctx context.Context, providerID RegistryProviderID, options RegistryProviderVersionCreateOptions) (*RegistryProviderVersion, error) {
	if err := providerID.valid(); err != nil {
		return nil, err
	}
	if providerID.RegistryName != PrivateRegistry {
		return nil, ErrRequiredPrivateRegistry
	}
	if err := options.valid(); err != nil {
		return nil, err
	}

	u := fmt.Sprintf(
		"organizations/%s/registry-providers/%s/%s/%s/versions",
		url.QueryEscape(providerID.OrganizationName),
		url.QueryEscape(string(providerID.RegistryName)),
		url.QueryEscape(providerID.Namespace),
		url.QueryEscape(providerID.Name),
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

// Read a registry provider
func (r *registryProviderVersions) Read(ctx context.Context, versionID RegistryProviderVersionID, options *RegistryProviderVersionReadOptions) (*RegistryProviderVersion, error) {
	if err := versionID.valid(); err != nil {
		return nil, err
	}

	u := fmt.Sprintf(
		"organizations/%s/registry-providers/%s/%s/%s/versions/%s",
		url.QueryEscape(versionID.OrganizationName),
		url.QueryEscape(string(versionID.RegistryName)),
		url.QueryEscape(versionID.Namespace),
		url.QueryEscape(versionID.Name),
		url.QueryEscape(versionID.Version),
	)
	req, err := r.client.newRequest("GET", u, options)
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

// Delete a registry provider
func (r *registryProviderVersions) Delete(ctx context.Context, versionID RegistryProviderVersionID) error {
	if err := versionID.valid(); err != nil {
		return err
	}

	u := fmt.Sprintf(
		"organizations/%s/registry-providers/%s/%s/%s/versions/%s",
		url.QueryEscape(versionID.OrganizationName),
		url.QueryEscape(string(versionID.RegistryName)),
		url.QueryEscape(versionID.Namespace),
		url.QueryEscape(versionID.Name),
		url.QueryEscape(versionID.Version),
	)
	req, err := r.client.newRequest("DELETE", u, nil)
	if err != nil {
		return err
	}

	return r.client.do(ctx, req, nil)
}

func (v RegistryProviderVersion) ShasumsUploadURL() (string, error) {
	uploadURL, ok := v.Links["shasums-upload"].(string)
	if !ok {
		return uploadURL, fmt.Errorf("the Registry Provider Version does not contain a shasums upload link")
	}
	if uploadURL == "" {
		return uploadURL, fmt.Errorf("the Registry Provider Version shasums upload URL is empty")
	}
	return uploadURL, nil
}

func (v RegistryProviderVersion) ShasumsSigUploadURL() (string, error) {
	uploadURL, ok := v.Links["shasums-sig-upload"].(string)
	if !ok {
		return uploadURL, fmt.Errorf("the Registry Provider Version does not contain a shasums sig upload link")
	}
	if uploadURL == "" {
		return uploadURL, fmt.Errorf("the Registry Provider Version shasums sig upload URL is empty")
	}
	return uploadURL, nil
}

func (v RegistryProviderVersion) ShasumsDownloadURL() (string, error) {
	downloadURL, ok := v.Links["shasums-download"].(string)
	if !ok {
		return downloadURL, fmt.Errorf("the Registry Provider Version does not contain a shasums download link")
	}
	if downloadURL == "" {
		return downloadURL, fmt.Errorf("the Registry Provider Version shasums download URL is empty")
	}
	return downloadURL, nil
}

func (v RegistryProviderVersion) ShasumsSigDownloadURL() (string, error) {
	downloadURL, ok := v.Links["shasums-sig-download"].(string)
	if !ok {
		return downloadURL, fmt.Errorf("the Registry Provider Version does not contain a shasums sig download link")
	}
	if downloadURL == "" {
		return downloadURL, fmt.Errorf("the Registry Provider Version shasums sig download URL is empty")
	}
	return downloadURL, nil
}

func (id RegistryProviderVersionID) valid() error {
	if !validStringID(&id.Version) {
		return ErrInvalidVersion
	}
	if id.RegistryName != PrivateRegistry {
		return ErrRequiredPrivateRegistry
	}
	if err := id.RegistryProviderID.valid(); err != nil {
		return err
	}
	return nil
}

func (o RegistryProviderVersionListOptions) valid() error {
	return nil
}

func (o RegistryProviderVersionCreateOptions) valid() error {
	if !validStringID(&o.Version) {
		return ErrInvalidVersion
	}
	if !validStringID(&o.KeyID) {
		return ErrInvalidKeyID
	}
	return nil
}
