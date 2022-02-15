package tfe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryProvidersCreate(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	orgTest, orgTestCleanup := createOrganization(t, client)
	defer orgTestCleanup()

	publicName := Public
	privateName := Private

	t.Run("with valid options", func(t *testing.T) {

		publicProviderOptions := RegistryProviderCreateOptions{
			Name:         String("provider_name"),
			Namespace:    String("public_namespace"),
			RegistryName: &publicName,
		}
		privateProviderOptions := RegistryProviderCreateOptions{
			Name:         String("provider_name"),
			Namespace:    &orgTest.Name,
			RegistryName: &privateName,
		}

		registryOptions := []RegistryProviderCreateOptions{publicProviderOptions, privateProviderOptions}

		for _, options := range registryOptions {
			prv, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			require.NoError(t, err)
			assert.NotEmpty(t, prv.ID)
			assert.Equal(t, *options.Name, prv.Name)
			assert.Equal(t, *options.Namespace, prv.Namespace)
			assert.Equal(t, *options.RegistryName, prv.RegistryName)

			t.Run("permissions are properly decoded", func(t *testing.T) {
				assert.True(t, prv.Permissions.CanDelete)
			})

			t.Run("relationships are properly decoded", func(t *testing.T) {
				assert.Equal(t, orgTest.Name, prv.Organization.Name)
			})

			t.Run("timestamps are properly decoded", func(t *testing.T) {
				assert.NotEmpty(t, prv.CreatedAt)
				assert.NotEmpty(t, prv.UpdatedAt)
			})

		}
	})

	t.Run("with invalid options", func(t *testing.T) {
		t.Run("without a name", func(t *testing.T) {
			options := RegistryProviderCreateOptions{
				Namespace:    String("namespace"),
				RegistryName: &publicName,
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, ErrRequiredName.Error())
		})

		t.Run("with an invalid name", func(t *testing.T) {
			options := RegistryProviderCreateOptions{
				Name:         String("invalid name"),
				Namespace:    String("namespace"),
				RegistryName: &publicName,
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, ErrInvalidName.Error())
		})

		t.Run("without a namespace", func(t *testing.T) {
			options := RegistryProviderCreateOptions{
				Name:         String("name"),
				RegistryName: &publicName,
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, "namespace is required")
		})

		t.Run("with an invalid namespace", func(t *testing.T) {
			options := RegistryProviderCreateOptions{
				Name:         String("name"),
				Namespace:    String("invalid namespace"),
				RegistryName: &publicName,
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, "invalid value for namespace")
		})

		t.Run("without a registry-name", func(t *testing.T) {
			options := RegistryProviderCreateOptions{
				Name:      String("name"),
				Namespace: String("namespace"),
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, "registry-name is required")
		})
	})

	t.Run("without a valid organization", func(t *testing.T) {
		options := RegistryProviderCreateOptions{
			Name:         String("name"),
			Namespace:    String("namespace"),
			RegistryName: &publicName,
		}
		rm, err := client.RegistryProviders.Create(ctx, badIdentifier, options)
		assert.Nil(t, rm)
		assert.EqualError(t, err, ErrInvalidOrg.Error())
	})

	t.Run("without a matching namespace organization.name for private registry", func(t *testing.T) {
		options := RegistryProviderCreateOptions{
			Name:         String("name"),
			Namespace:    String("namespace"),
			RegistryName: &privateName,
		}
		rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
		assert.Nil(t, rm)
		assert.EqualError(t, err, "namespace must match organization name for private providers")
	})
}
