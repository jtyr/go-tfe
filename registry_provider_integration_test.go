package tfe

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryProvidersList(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("with providers", func(t *testing.T) {
		orgTest, orgTestCleanup := createOrganization(t, client)
		defer orgTestCleanup()

		createN := 10
		providers := make([]*RegistryProvider, 0)
		// these providers will be destroyed when the org is cleaned up
		for i := 0; i < createN; i++ {
			providerTest, _ := createPublicRegistryProvider(t, client, orgTest)
			providers = append(providers, providerTest)
		}
		for i := 0; i < createN; i++ {
			providerTest, _ := createPrivateRegistryProvider(t, client, orgTest)
			providers = append(providers, providerTest)
		}
		providerN := len(providers)
		publicProviderN := createN

		t.Run("returns all providers", func(t *testing.T) {
			returnedProviders, err := client.RegistryProviders.List(ctx, orgTest.Name, &RegistryProviderListOptions{
				ListOptions: ListOptions{
					PageNumber: 0,
					PageSize:   providerN,
				},
			})
			require.NoError(t, err)
			assert.NotEmpty(t, returnedProviders.Items)
			assert.Equal(t, providerN, returnedProviders.TotalCount)
			assert.Equal(t, 1, returnedProviders.TotalPages)
			for _, rp := range returnedProviders.Items {
				foundProvider := false
				for _, p := range providers {
					if rp.ID == p.ID {
						foundProvider = true
						break
					}
				}
				assert.True(t, foundProvider, "Expected to find provider %s but did not:\nexpected:\n%v\nreturned\n%v", rp.ID, providers, returnedProviders)
			}
		})

		t.Run("returns pages", func(t *testing.T) {
			pageN := 2
			pageSize := providerN / pageN

			for page := 0; page < pageN; page++ {
				testName := fmt.Sprintf("returns page %d of providers", page)
				t.Run(testName, func(t *testing.T) {
					returnedProviders, err := client.RegistryProviders.List(ctx, orgTest.Name, &RegistryProviderListOptions{
						ListOptions: ListOptions{
							PageNumber: page,
							PageSize:   pageSize,
						},
					})
					require.NoError(t, err)
					assert.NotEmpty(t, returnedProviders.Items)
					assert.Equal(t, providerN, returnedProviders.TotalCount)
					assert.Equal(t, pageN, returnedProviders.TotalPages)
					assert.Equal(t, pageSize, len(returnedProviders.Items))
					for _, rp := range returnedProviders.Items {
						foundProvider := false
						for _, p := range providers {
							if rp.ID == p.ID {
								foundProvider = true
								break
							}
						}
						assert.True(t, foundProvider, "Expected to find provider %s but did not:\nexpected:\n%v\nreturned\n%v", rp.ID, providers, returnedProviders)
					}
				})
			}
		})

		t.Run("filters on registry name", func(t *testing.T) {
			returnedProviders, err := client.RegistryProviders.List(ctx, orgTest.Name, &RegistryProviderListOptions{
				RegistryName: PublicRegistry,
				ListOptions: ListOptions{
					PageNumber: 0,
					PageSize:   providerN,
				},
			})
			require.NoError(t, err)
			assert.NotEmpty(t, returnedProviders.Items)
			assert.Equal(t, publicProviderN, returnedProviders.TotalCount)
			assert.Equal(t, 1, returnedProviders.TotalPages)
			for _, rp := range returnedProviders.Items {
				foundProvider := false
				for _, p := range providers {
					if rp.ID == p.ID {
						foundProvider = true
						break
					}
				}
				assert.Equal(t, PublicRegistry, rp.RegistryName)
				assert.True(t, foundProvider, "Expected to find provider %s but did not:\nexpected:\n%v\nreturned\n%v", rp.ID, providers, returnedProviders)
			}
		})

		t.Run("searches", func(t *testing.T) {
			expectedProvider := providers[0]
			returnedProviders, err := client.RegistryProviders.List(ctx, orgTest.Name, &RegistryProviderListOptions{
				Search: expectedProvider.Name,
				ListOptions: ListOptions{
					PageNumber: 0,
					PageSize:   providerN,
				},
			})
			require.NoError(t, err)
			assert.NotEmpty(t, returnedProviders.Items)
			assert.Equal(t, 1, returnedProviders.TotalCount)
			assert.Equal(t, 1, returnedProviders.TotalPages)
			foundProvider := returnedProviders.Items[0]

			assert.Equal(t, foundProvider.ID, expectedProvider.ID, "Expected to find provider %s but did not:\nexpected:\n%v\nreturned\n%v", expectedProvider.ID, providers, returnedProviders)
		})
	})

	t.Run("without providers", func(t *testing.T) {
		orgTest, orgTestCleanup := createOrganization(t, client)
		defer orgTestCleanup()

		providers, err := client.RegistryProviders.List(ctx, orgTest.Name, nil)
		require.NoError(t, err)
		assert.Empty(t, providers.Items)
		assert.Equal(t, 0, providers.TotalCount)
		assert.Equal(t, 0, providers.TotalPages)
	})
}

func TestRegistryProvidersCreate(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	orgTest, orgTestCleanup := createOrganization(t, client)
	defer orgTestCleanup()

	t.Run("with valid options", func(t *testing.T) {

		publicProviderOptions := RegistryProviderCreateOptions{
			Name:         "provider_name",
			Namespace:    "public_namespace",
			RegistryName: PublicRegistry,
		}
		privateProviderOptions := RegistryProviderCreateOptions{
			Name:         "provider_name",
			Namespace:    orgTest.Name,
			RegistryName: PrivateRegistry,
		}

		registryOptions := []RegistryProviderCreateOptions{publicProviderOptions, privateProviderOptions}

		for _, options := range registryOptions {
			testName := fmt.Sprintf("with %s provider", options.RegistryName)
			t.Run(testName, func(t *testing.T) {
				prv, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
				require.NoError(t, err)
				assert.NotEmpty(t, prv.ID)
				assert.Equal(t, options.Name, prv.Name)
				assert.Equal(t, options.Namespace, prv.Namespace)
				assert.Equal(t, options.RegistryName, prv.RegistryName)

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
				Namespace:    "namespace",
				RegistryName: PublicRegistry,
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, ErrRequiredName.Error())
		})

		t.Run("with an invalid name", func(t *testing.T) {
			options := RegistryProviderCreateOptions{
				Name:         "invalid name",
				Namespace:    "namespace",
				RegistryName: PublicRegistry,
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, ErrInvalidName.Error())
		})

		t.Run("without a namespace", func(t *testing.T) {
			options := RegistryProviderCreateOptions{
				Name:         "name",
				RegistryName: PublicRegistry,
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, "namespace is required")
		})

		t.Run("with an invalid namespace", func(t *testing.T) {
			options := RegistryProviderCreateOptions{
				Name:         "name",
				Namespace:    "invalid namespace",
				RegistryName: PublicRegistry,
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, "invalid value for namespace")
		})

		t.Run("without a registry-name", func(t *testing.T) {
			options := RegistryProviderCreateOptions{
				Name:      "name",
				Namespace: "namespace",
			}
			rm, err := client.RegistryProviders.Create(ctx, orgTest.Name, options)
			assert.Nil(t, rm)
			assert.EqualError(t, err, "registry-name is required")
		})
	})

	t.Run("without a valid organization", func(t *testing.T) {
		options := RegistryProviderCreateOptions{
			Name:         "name",
			Namespace:    "namespace",
			RegistryName: PublicRegistry,
		}
		rm, err := client.RegistryProviders.Create(ctx, badIdentifier, options)
		assert.Nil(t, rm)
		assert.EqualError(t, err, ErrInvalidOrg.Error())
	})

	t.Run("without a matching namespace organization.name for private registry", func(t *testing.T) {
		options := RegistryProviderCreateOptions{
			Name:         "name",
			Namespace:    "namespace",
			RegistryName: PrivateRegistry,
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
			RegistryName:    PublicRegistry,
		},
		{
			ProviderCreator: createPrivateRegistryProvider,
			RegistryName:    PrivateRegistry,
		},
	}

	for _, prvCtx := range providerContexts {
		testName := fmt.Sprintf("with %s provider", prvCtx.RegistryName)
		t.Run(testName, func(t *testing.T) {
			t.Run("with valid provider", func(t *testing.T) {
				registryProviderTest, providerTestCleanup := prvCtx.ProviderCreator(t, client, orgTest)
				defer providerTestCleanup()

				id := RegistryProviderID{
					OrganizationName: orgTest.Name,
					RegistryName:     registryProviderTest.RegistryName,
					Namespace:        registryProviderTest.Namespace,
					Name:             registryProviderTest.Name,
				}

				prv, err := client.RegistryProviders.Read(ctx, id, nil)
				assert.NoError(t, err)
				assert.NotEmpty(t, prv.ID)
				assert.Equal(t, registryProviderTest.Name, prv.Name)
				assert.Equal(t, registryProviderTest.Namespace, prv.Namespace)
				assert.Equal(t, registryProviderTest.RegistryName, prv.RegistryName)

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

			t.Run("when the registry provider does not exist", func(t *testing.T) {
				id := RegistryProviderID{
					OrganizationName: orgTest.Name,
					RegistryName:     prvCtx.RegistryName,
					Namespace:        "nonexistent",
					Name:             "nonexistent",
				}
				_, err := client.RegistryProviders.Read(ctx, id, nil)
				assert.Error(t, err)
				// Local TFC/E will return a forbidden here when TFC/E is in development mode
				// In non development mode this returns a 404
				assert.Equal(t, ErrResourceNotFound, err)
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
			RegistryName:    PublicRegistry,
		},
		{
			ProviderCreator: createPrivateRegistryProvider,
			RegistryName:    PrivateRegistry,
		},
	}

	for _, prvCtx := range providerContexts {
		testName := fmt.Sprintf("with %s provider", prvCtx.RegistryName)
		t.Run(testName, func(t *testing.T) {
			t.Run("with valid provider", func(t *testing.T) {
				registryProviderTest, _ := prvCtx.ProviderCreator(t, client, orgTest)

				id := RegistryProviderID{
					OrganizationName: orgTest.Name,
					RegistryName:     registryProviderTest.RegistryName,
					Namespace:        registryProviderTest.Namespace,
					Name:             registryProviderTest.Name,
				}

				err := client.RegistryProviders.Delete(ctx, id)
				require.NoError(t, err)

				prv, err := client.RegistryProviders.Read(ctx, id, nil)
				assert.Nil(t, prv)
				assert.Error(t, err)
			})

			t.Run("when the registry provider does not exist", func(t *testing.T) {
				id := RegistryProviderID{
					OrganizationName: orgTest.Name,
					RegistryName:     prvCtx.RegistryName,
					Namespace:        "nonexistent",
					Name:             "nonexistent",
				}
				err := client.RegistryProviders.Delete(ctx, id)
				assert.Error(t, err)
				// Local TFC/E will return a forbidden here when TFC/E is in development mode
				// In non development mode this returns a 404
				assert.Equal(t, ErrResourceNotFound, err)
			})
		})
	}
}

func TestRegistryProvidersIDValidation(t *testing.T) {
	orgName := "orgName"
	registryName := PublicRegistry

	t.Run("valid", func(t *testing.T) {
		id := RegistryProviderID{
			OrganizationName: orgName,
			RegistryName:     registryName,
			Namespace:        "namespace",
			Name:             "name",
		}
		assert.NoError(t, id.valid())
	})

	t.Run("without a name", func(t *testing.T) {
		id := RegistryProviderID{
			OrganizationName: orgName,
			RegistryName:     registryName,
			Namespace:        "namespace",
			Name:             "",
		}
		assert.EqualError(t, id.valid(), ErrRequiredName.Error())
	})

	t.Run("with an invalid name", func(t *testing.T) {
		id := RegistryProviderID{
			OrganizationName: orgName,
			RegistryName:     registryName,
			Namespace:        "namespace",
			Name:             badIdentifier,
		}
		assert.EqualError(t, id.valid(), ErrInvalidName.Error())
	})

	t.Run("without a namespace", func(t *testing.T) {
		id := RegistryProviderID{
			OrganizationName: orgName,
			RegistryName:     registryName,
			Namespace:        "",
			Name:             "name",
		}
		assert.EqualError(t, id.valid(), "namespace is required")
	})

	t.Run("with an invalid namespace", func(t *testing.T) {
		id := RegistryProviderID{
			OrganizationName: orgName,
			RegistryName:     registryName,
			Namespace:        badIdentifier,
			Name:             "name",
		}
		assert.EqualError(t, id.valid(), "invalid value for namespace")
	})

	t.Run("without a registry-name", func(t *testing.T) {
		id := RegistryProviderID{
			OrganizationName: orgName,
			RegistryName:     "",
			Namespace:        "namespace",
			Name:             "name",
		}
		assert.EqualError(t, id.valid(), "registry-name is required")
	})

	t.Run("without a valid organization", func(t *testing.T) {
		id := RegistryProviderID{
			OrganizationName: badIdentifier,
			RegistryName:     registryName,
			Namespace:        "namespace",
			Name:             "name",
		}
		assert.EqualError(t, id.valid(), ErrInvalidOrg.Error())
	})
}
