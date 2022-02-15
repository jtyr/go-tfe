package tfe

import (
	"context"
	"fmt"
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
			testName := fmt.Sprintf("with %s provider", *options.RegistryName)
			t.Run(testName, func(t *testing.T) {
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

func TestRegistryProvidersRead(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	orgTest, orgTestCleanup := createOrganization(t, client)
	defer orgTestCleanup()

	type ProviderContext struct {
		ProviderCreator func(t *testing.T, client *Client, org *Organization) (*RegistryProvider, func())
		RegistryName    RegistryName
	}

	providerContexts := []ProviderContext{
		{
			ProviderCreator: createPublicRegistryProvider,
			RegistryName:    Public,
		},
		{
			ProviderCreator: createPrivateRegistryProvider,
			RegistryName:    Private,
		},
	}

	for _, prvCtx := range providerContexts {
		testName := fmt.Sprintf("with %s provider", prvCtx.RegistryName)
		t.Run(testName, func(t *testing.T) {
			t.Run("with valid provider", func(t *testing.T) {
				registryProviderTest, providerTestCleanup := prvCtx.ProviderCreator(t, client, orgTest)
				defer providerTestCleanup()

				prv, err := client.RegistryProviders.Read(ctx, orgTest.Name, registryProviderTest.RegistryName, registryProviderTest.Namespace, registryProviderTest.Name, nil)
				assert.NoError(t, err)
				assert.NotEmpty(t, prv.ID)
				assert.Equal(t, registryProviderTest.Name, prv.Name)
				assert.Equal(t, registryProviderTest.Namespace, prv.Namespace)
				assert.Equal(t, registryProviderTest.RegistryName, prv.RegistryName)
			})

			t.Run("when the registry provider does not exist", func(t *testing.T) {
				_, err := client.RegistryProviders.Read(ctx, orgTest.Name, prvCtx.RegistryName, "nonexistent", "nonexistent", nil)
				assert.Error(t, err)
				// Local TFC/E will return a forbidden here when TFC/E is in development mode
				// In non development mode this returns a 404
				assert.Equal(t, ErrResourceNotFound, err)
			})

			t.Run("without a name", func(t *testing.T) {
				_, err := client.RegistryProviders.Read(ctx, orgTest.Name, prvCtx.RegistryName, "namespace", "", nil)
				assert.EqualError(t, err, ErrRequiredName.Error())
			})

			t.Run("with an invalid name", func(t *testing.T) {
				_, err := client.RegistryProviders.Read(ctx, orgTest.Name, prvCtx.RegistryName, "namespace", badIdentifier, nil)
				assert.EqualError(t, err, ErrInvalidName.Error())
			})

			t.Run("without a namespace", func(t *testing.T) {
				_, err := client.RegistryProviders.Read(ctx, orgTest.Name, prvCtx.RegistryName, "", "name", nil)
				assert.EqualError(t, err, "namespace is required")
			})

			t.Run("with an invalid namespace", func(t *testing.T) {
				_, err := client.RegistryProviders.Read(ctx, orgTest.Name, prvCtx.RegistryName, badIdentifier, "name", nil)
				assert.EqualError(t, err, "invalid value for namespace")
			})

			t.Run("without a registry-name", func(t *testing.T) {
				_, err := client.RegistryProviders.Read(ctx, orgTest.Name, "", "namespace", "name", nil)
				assert.EqualError(t, err, "registry-name is required")
			})

			t.Run("without a valid organization", func(t *testing.T) {
				_, err := client.RegistryProviders.Read(ctx, badIdentifier, prvCtx.RegistryName, "namespace", "name", nil)
				assert.EqualError(t, err, ErrInvalidOrg.Error())
			})
		})
	}
}

func TestRegistryProvidersDelete(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	orgTest, orgTestCleanup := createOrganization(t, client)
	defer orgTestCleanup()

	type ProviderContext struct {
		ProviderCreator func(t *testing.T, client *Client, org *Organization) (*RegistryProvider, func())
		RegistryName    RegistryName
	}

	providerContexts := []ProviderContext{
		{
			ProviderCreator: createPublicRegistryProvider,
			RegistryName:    Public,
		},
		{
			ProviderCreator: createPrivateRegistryProvider,
			RegistryName:    Private,
		},
	}

	for _, prvCtx := range providerContexts {
		testName := fmt.Sprintf("with %s provider", prvCtx.RegistryName)
		t.Run(testName, func(t *testing.T) {
			t.Run("with valid provider", func(t *testing.T) {
				registryProviderTest, _ := prvCtx.ProviderCreator(t, client, orgTest)

				err := client.RegistryProviders.Delete(ctx, orgTest.Name, registryProviderTest.RegistryName, registryProviderTest.Namespace, registryProviderTest.Name)
				require.NoError(t, err)

				prv, err := client.RegistryProviders.Read(ctx, orgTest.Name, registryProviderTest.RegistryName, registryProviderTest.Namespace, registryProviderTest.Name, nil)
				assert.Nil(t, prv)
				assert.Error(t, err)
			})

			t.Run("when the registry provider does not exist", func(t *testing.T) {
				err := client.RegistryProviders.Delete(ctx, orgTest.Name, prvCtx.RegistryName, "nonexistent", "nonexistent")
				assert.Error(t, err)
				// Local TFC/E will return a forbidden here when TFC/E is in development mode
				// In non development mode this returns a 404
				assert.Equal(t, ErrResourceNotFound, err)
			})

			t.Run("without a name", func(t *testing.T) {
				err := client.RegistryProviders.Delete(ctx, orgTest.Name, prvCtx.RegistryName, "namespace", "")
				assert.EqualError(t, err, ErrRequiredName.Error())
			})

			t.Run("with an invalid name", func(t *testing.T) {
				err := client.RegistryProviders.Delete(ctx, orgTest.Name, prvCtx.RegistryName, "namespace", badIdentifier)
				assert.EqualError(t, err, ErrInvalidName.Error())
			})

			t.Run("without a namespace", func(t *testing.T) {
				err := client.RegistryProviders.Delete(ctx, orgTest.Name, prvCtx.RegistryName, "", "name")
				assert.EqualError(t, err, "namespace is required")
			})

			t.Run("with an invalid namespace", func(t *testing.T) {
				err := client.RegistryProviders.Delete(ctx, orgTest.Name, prvCtx.RegistryName, badIdentifier, "name")
				assert.EqualError(t, err, "invalid value for namespace")
			})

			t.Run("without a registry-name", func(t *testing.T) {
				err := client.RegistryProviders.Delete(ctx, orgTest.Name, "", "namespace", "name")
				assert.EqualError(t, err, "registry-name is required")
			})

			t.Run("without a valid organization", func(t *testing.T) {
				err := client.RegistryProviders.Delete(ctx, badIdentifier, prvCtx.RegistryName, "namespace", "name")
				assert.EqualError(t, err, ErrInvalidOrg.Error())
			})
		})
	}
}
