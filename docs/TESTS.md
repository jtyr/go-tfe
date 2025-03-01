## Running tests

### 1. (Optional) Create repositories for policy sets and registry modules

If you are planning to run the full suite of tests or work on policy sets or registry modules, you'll need to set up repositories for them in GitHub.

Your policy set repository will need the following: 
1. A policy set stored in a subdirectory `policy-sets/foo`
1. A branch other than `main` named `policies`

Your registry module repository will need to be a [valid module](https://www.terraform.io/docs/cloud/registry/publish.html#preparing-a-module-repository).
It will need the following: 
1. To be named `terraform-<PROVIDER>-<NAME>`
1. At least one valid SemVer tag in the format `x.y.z`
[terraform-random-module](ttps://github.com/caseylang/terraform-random-module) is a good example repo. 
   
### 2. Set up environment variables

##### Required:
Tests are run against an actual backend so they require a valid backend address and token.
1. `TFE_ADDRESS` - URL of a Terraform Cloud or Terraform Enterprise instance to be used for testing, including scheme. Example: `https://tfe.local`
1. `TFE_TOKEN` - A [user API token](https://www.terraform.io/docs/cloud/users-teams-organizations/users.html#api-tokens) for the Terraform Cloud or Terraform Enterprise instance being used for testing.

**Note:** Alternatively, you can set `TFE_HOSTNAME` which serves as a fallback for `TFE_ADDRESS`. It will only be used if `TFE_ADDRESS` is not set and will resolve the host to an `https` scheme. Example: `tfe.local` => resolves to `https://tfe.local`

##### Optional:
1. `GITHUB_TOKEN` - [GitHub personal access token](https://help.github.com/en/github/authenticating-to-github/creating-a-personal-access-token-for-the-command-line). Required for running any tests that use VCS (OAuth clients, policy sets, etc).
1. `GITHUB_POLICY_SET_IDENTIFIER` - GitHub policy set repository identifier in the format `username/repository`. Required for running policy set tests.
1. `GITHUB_REGISTRY_MODULE_IDENTIFIER` - GitHub registry module repository identifier in the format `username/repository`. Required for running registry module tests.
1. `ENABLE_TFE` - Some tests are only applicable to Terraform Enterprise or Terraform Cloud. By setting `ENABLE_TFE=1` you will enable enterprise only tests and disable cloud only tests. In CI `ENABLE_TFE` is not set so if you are writing enterprise only features you should manually test with `ENABLE_TFE=1` against a Terraform Enterprise instance.
1. `SKIP_PAID` - Some tests depend on paid only features. By setting `SKIP_PAID=1`, you will skip tests that access paid features.
1. `ENABLE_BETA` - Some tests require access to beta features. By setting `ENABLE_BETA=1` you will enable tests that require access to beta features. IN CI `ENABLE_BETA` is not set so if you are writing beta only features you should manually test with `ENABLE_BETA=1` against a Terraform Enterprise instance with those features enabled.  
1. `TFC_RUN_TASK_URL` - Run task integration tests require a URL to use when creating run tasks. To learn more about the Run Task API, [read here](https://www.terraform.io/cloud-docs/api-docs/run-tasks)

You can set your environment variables up however you prefer. The following are instructions for setting up environment variables using [envchain](https://github.com/sorah/envchain).
   1. Make sure you have envchain installed. [Instructions for this can be found in the envchain README](https://github.com/sorah/envchain#installation).
   1. Pick a namespace for storing your environment variables. I suggest `go-tfe` or something similar.
   1. For each environment variable you need to set, run the following command:
      ```sh
      envchain --set YOUR_NAMESPACE_HERE ENVIRONMENT_VARIABLE_HERE
      ```
      **OR**
    
      Set all of the environment variables at once with the following command:
      ```sh
      envchain --set YOUR_NAMESPACE_HERE TFE_ADDRESS TFE_TOKEN GITHUB_TOKEN GITHUB_POLICY_SET_IDENTIFIER
      ```

### 3. Make sure run queue settings are correct

In order for the tests relating to queuing and capacity to pass, FRQ (fair run queuing) should be
enabled with a limit of 2 concurrent runs per organization on the Terraform Cloud or Terraform Enterprise instance you are using for testing.

### 4. Run the tests

#### Running all the tests
As running the all of the tests takes about ~20 minutes, make sure to add a timeout to your
command (as the default timeout is 10m).

##### With envchain:
```sh
$ envchain YOUR_NAMESPACE_HERE go test ./... -timeout=30m -tags=integration
```

##### Without envchain:
```sh
$ TFE_TOKEN=xyz TFE_ADDRESS=xyz ENABLE_TFE=1 go test ./... -timeout=30m -tags=integration
```

#### Running specific tests

The commands below use notification configurations as an example.

##### With envchain:
```sh
$ envchain YOUR_NAMESPACE_HERE go test -run TestNotificationConfiguration -v ./... -tags=integration
```

##### Without envchain:
```sh
$ TFE_TOKEN=xyz TFE_ADDRESS=xyz ENABLE_TFE=1 go test -run TestNotificationConfiguration -v ./... -tags=integration
```   

