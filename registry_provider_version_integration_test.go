package tfe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryProviderVersionsIDValidation(t *testing.T) {
	version := "1.0.0"
	validRegistryProviderId := RegistryProviderID{
		OrganizationName: "orgName",
		RegistryName:     PrivateRegistry,
		Namespace:        "namespace",
		Name:             "name",
	}
	invalidRegistryProviderId := RegistryProviderID{
		OrganizationName: badIdentifier,
		RegistryName:     PrivateRegistry,
		Namespace:        "namespace",
		Name:             "name",
	}
	publicRegistryProviderId := RegistryProviderID{
		OrganizationName: "orgName",
		RegistryName:     PublicRegistry,
		Namespace:        "namespace",
		Name:             "name",
	}

	t.Run("valid", func(t *testing.T) {
		id := RegistryProviderVersionID{
			Version:            version,
			RegistryProviderID: validRegistryProviderId,
		}
		assert.NoError(t, id.valid())
	})

	t.Run("without a version", func(t *testing.T) {
		id := RegistryProviderVersionID{
			Version:            "",
			RegistryProviderID: validRegistryProviderId,
		}
		assert.EqualError(t, id.valid(), "version is required")
	})

	t.Run("without a key-id", func(t *testing.T) {
		id := RegistryProviderVersionID{
			Version:            "",
			RegistryProviderID: validRegistryProviderId,
		}
		assert.EqualError(t, id.valid(), "version is required")
	})

	t.Run("invalid version", func(t *testing.T) {
		t.Skip("This is skipped as we don't actually validate version is a valid semver")
		id := RegistryProviderVersionID{
			Version:            "foo",
			RegistryProviderID: validRegistryProviderId,
		}
		assert.EqualError(t, id.valid(), "version is required")
	})

	t.Run("invalid registry for parent provider", func(t *testing.T) {
		id := RegistryProviderVersionID{
			Version:            version,
			RegistryProviderID: publicRegistryProviderId,
		}
		assert.EqualError(t, id.valid(), "only private registry is allowed")
	})

	t.Run("without a valid registry provider id", func(t *testing.T) {
		// this is a proxy for all permutations of an invalid registry provider id
		// it is assumed that validity of the registry provider id is delegated to its own valid method
		id := RegistryProviderVersionID{
			Version:            version,
			RegistryProviderID: invalidRegistryProviderId,
		}
		assert.EqualError(t, id.valid(), ErrInvalidOrg.Error())
	})
}

func TestRegistryProviderVersionsCreate(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	providerTest, providerTestCleanup := createPrivateRegistryProvider(t, client, nil)
	defer providerTestCleanup()

	providerId := RegistryProviderID{
		OrganizationName: providerTest.Organization.Name,
		RegistryName:     providerTest.RegistryName,
		Namespace:        providerTest.Namespace,
		Name:             providerTest.Name,
	}

	t.Run("with valid options", func(t *testing.T) {
		options := RegistryProviderVersionCreateOptions{
			Version: "1.0.0",
			KeyID:   "abcdefg",
		}
		prvv, err := client.RegistryProviderVersions.Create(ctx, providerId, options)
		require.NoError(t, err)
		assert.NotEmpty(t, prvv.ID)
		assert.Equal(t, options.Version, prvv.Version)
		assert.Equal(t, options.KeyID, prvv.KeyID)

		t.Run("relationships are properly decoded", func(t *testing.T) {
			assert.Equal(t, providerTest.ID, prvv.RegistryProvider.ID)
		})

		t.Run("timestamps are properly decoded", func(t *testing.T) {
			assert.NotEmpty(t, prvv.CreatedAt)
			assert.NotEmpty(t, prvv.UpdatedAt)
		})

		t.Run("includes upload links", func(t *testing.T) {
			expectedLinks := []string{
				"shasums-upload",
				"shasums-sig-upload",
			}
			for _, l := range expectedLinks {
				_, ok := prvv.Links[l].(string)
				assert.True(t, ok, "Expect upload link: %s", l)
			}
		})
	})

	t.Run("with invalid options", func(t *testing.T) {
		t.Run("without a version", func(t *testing.T) {
			options := RegistryProviderVersionCreateOptions{
				Version: "",
				KeyID:   "abcdefg",
			}
			rm, err := client.RegistryProviderVersions.Create(ctx, providerId, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, "version is required")
		})

		t.Run("without a key-id", func(t *testing.T) {
			options := RegistryProviderVersionCreateOptions{
				Version: "1.0.0",
				KeyID:   "",
			}
			rm, err := client.RegistryProviderVersions.Create(ctx, providerId, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, "key-id is required")
		})

		t.Run("with a public provider", func(t *testing.T) {
			options := RegistryProviderVersionCreateOptions{
				Version: "1.0.0",
				KeyID:   "abcdefg",
			}
			providerId := RegistryProviderID{
				OrganizationName: providerTest.Organization.Name,
				RegistryName:     PublicRegistry,
				Namespace:        providerTest.Namespace,
				Name:             providerTest.Name,
			}
			rm, err := client.RegistryProviderVersions.Create(ctx, providerId, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, "only private registry is allowed")
		})

		t.Run("without a valid provider id", func(t *testing.T) {
			options := RegistryProviderVersionCreateOptions{
				Version: "1.0.0",
				KeyID:   "abcdefg",
			}
			providerId := RegistryProviderID{
				OrganizationName: badIdentifier,
				RegistryName:     providerTest.RegistryName,
				Namespace:        providerTest.Namespace,
				Name:             providerTest.Name,
			}
			rm, err := client.RegistryProviderVersions.Create(ctx, providerId, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, ErrInvalidOrg.Error())
		})
	})

	t.Run("without a valid provider id", func(t *testing.T) {
	})
}
