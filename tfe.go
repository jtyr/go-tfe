package tfe

import (
	"errors"
	"io/fs"
	"log"
	"sort"

	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	cleanhttp "github.com/hashicorp/go-cleanhttp"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/jsonapi"
	"golang.org/x/time/rate"

	slug "github.com/hashicorp/go-slug"
)

const (
	_userAgent         = "go-tfe"
	_headerRateLimit   = "X-RateLimit-Limit"
	_headerRateReset   = "X-RateLimit-Reset"
	_headerAPIVersion  = "TFP-API-Version"
	_includeQueryParam = "include"

	DefaultAddress  = "https://app.terraform.io"
	DefaultBasePath = "/api/v2/"
	// PingEndpoint is a no-op API endpoint used to configure the rate limiter
	PingEndpoint = "ping"
)

// RetryLogHook allows a function to run before each retry.

type RetryLogHook func(attemptNum int, resp *http.Response)

// Config provides configuration details to the API client.

type Config struct {
	// The address of the Terraform Enterprise API.
	Address string

	// The base path on which the API is served.
	BasePath string

	// API token used to access the Terraform Enterprise API.
	Token string

	// Headers that will be added to every request.
	Headers http.Header

	// A custom HTTP client to use.
	HTTPClient *http.Client

	// RetryLogHook is invoked each time a request is retried.
	RetryLogHook RetryLogHook
}

// DefaultConfig returns a default config structure.

func DefaultConfig() *Config {
	config := &Config{
		Address:    os.Getenv("TFE_ADDRESS"),
		BasePath:   DefaultBasePath,
		Token:      os.Getenv("TFE_TOKEN"),
		Headers:    make(http.Header),
		HTTPClient: cleanhttp.DefaultPooledClient(),
	}

	// Set the default address if none is given.
	if config.Address == "" {
		if host := os.Getenv("TFE_HOSTNAME"); host != "" {
			config.Address = fmt.Sprintf("https://%s", host)
		} else {
			config.Address = DefaultAddress
		}
	}

	// Set the default user agent.
	config.Headers.Set("User-Agent", _userAgent)

	return config
}

// Client is the Terraform Enterprise API client. It provides the basic
// connectivity and configuration for accessing the TFE API
type Client struct {
	baseURL           *url.URL
	token             string
	headers           http.Header
	http              *retryablehttp.Client
	limiter           *rate.Limiter
	retryLogHook      RetryLogHook
	retryServerErrors bool
	remoteAPIVersion  string

	Admin                      Admin
	AgentPools                 AgentPools
	AgentTokens                AgentTokens
	Applies                    Applies
	Comments                   Comments
	ConfigurationVersions      ConfigurationVersions
	CostEstimates              CostEstimates
	NotificationConfigurations NotificationConfigurations
	OAuthClients               OAuthClients
	OAuthTokens                OAuthTokens
	Organizations              Organizations
	OrganizationMemberships    OrganizationMemberships
	OrganizationTags           OrganizationTags
	OrganizationTokens         OrganizationTokens
	Plans                      Plans
	PlanExports                PlanExports
	Policies                   Policies
	PolicyChecks               PolicyChecks
	PolicySetParameters        PolicySetParameters
	PolicySetVersions          PolicySetVersions
	PolicySets                 PolicySets
	RegistryModules            RegistryModules
	Runs                       Runs
	RunTasks                   RunTasks
	RunTriggers                RunTriggers
	SSHKeys                    SSHKeys
	StateVersionOutputs        StateVersionOutputs
	StateVersions              StateVersions
	TaskResults                TaskResults
	TaskStages                 TaskStages
	Teams                      Teams
	TeamAccess                 TeamAccesses
	TeamMembers                TeamMembers
	TeamTokens                 TeamTokens
	Users                      Users
	UserTokens                 UserTokens
	Variables                  Variables
	VariableSets               VariableSets
	VariableSetVariables       VariableSetVariables
	Workspaces                 Workspaces
	WorkspaceRunTasks          WorkspaceRunTasks

	Meta Meta
}

// Admin is the the Terraform Enterprise Admin API. It provides access to site
// wide admin settings. These are only available for Terraform Enterprise and
// do not function against Terraform Cloud
type Admin struct {
	Organizations     AdminOrganizations
	Workspaces        AdminWorkspaces
	Runs              AdminRuns
	TerraformVersions AdminTerraformVersions
	Users             AdminUsers
	Settings          *AdminSettings
}

// Meta contains any Terraform Cloud APIs which provide data about the API itself.
type Meta struct {
	IPRanges IPRanges
}

// NewClient creates a new Terraform Enterprise API client.
func NewClient(cfg *Config) (*Client, error) {
	config := DefaultConfig()

	// Layer in the provided config for any non-blank values.
	if cfg != nil { // nolint
		if cfg.Address != "" {
			config.Address = cfg.Address
		}
		if cfg.BasePath != "" {
			config.BasePath = cfg.BasePath
		}
		if cfg.Token != "" {
			config.Token = cfg.Token
		}
		for k, v := range cfg.Headers {
			config.Headers[k] = v
		}
		if cfg.HTTPClient != nil {
			config.HTTPClient = cfg.HTTPClient
		}
		if cfg.RetryLogHook != nil {
			config.RetryLogHook = cfg.RetryLogHook
		}
	}

	// Parse the address to make sure its a valid URL.
	baseURL, err := url.Parse(config.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	baseURL.Path = config.BasePath
	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path += "/"
	}

	// This value must be provided by the user.
	if config.Token == "" {
		return nil, fmt.Errorf("missing API token")
	}

	// Create the client.
	client := &Client{
		baseURL:      baseURL,
		token:        config.Token,
		headers:      config.Headers,
		retryLogHook: config.RetryLogHook,
	}

	client.http = &retryablehttp.Client{
		Backoff:      client.retryHTTPBackoff,
		CheckRetry:   client.retryHTTPCheck,
		ErrorHandler: retryablehttp.PassthroughErrorHandler,
		HTTPClient:   config.HTTPClient,
		RetryWaitMin: 100 * time.Millisecond,
		RetryWaitMax: 400 * time.Millisecond,
		RetryMax:     30,
	}

	meta, err := client.getRawAPIMetadata()
	if err != nil {
		return nil, err
	}

	// Configure the rate limiter.
	client.configureLimiter(meta.RateLimit)

	// Save the API version so we can return it from the RemoteAPIVersion
	// method later.
	client.remoteAPIVersion = meta.APIVersion

	// Create Admin
	client.Admin = Admin{
		Organizations:     &adminOrganizations{client: client},
		Workspaces:        &adminWorkspaces{client: client},
		Runs:              &adminRuns{client: client},
		Settings:          newAdminSettings(client),
		TerraformVersions: &adminTerraformVersions{client: client},
		Users:             &adminUsers{client: client},
	}

	// Create the services.
	client.AgentPools = &agentPools{client: client}
	client.AgentTokens = &agentTokens{client: client}
	client.Applies = &applies{client: client}
	client.Comments = &comments{client: client}
	client.ConfigurationVersions = &configurationVersions{client: client}
	client.CostEstimates = &costEstimates{client: client}
	client.NotificationConfigurations = &notificationConfigurations{client: client}
	client.OAuthClients = &oAuthClients{client: client}
	client.OAuthTokens = &oAuthTokens{client: client}
	client.Organizations = &organizations{client: client}
	client.OrganizationMemberships = &organizationMemberships{client: client}
	client.OrganizationTags = &organizationTags{client: client}
	client.OrganizationTokens = &organizationTokens{client: client}
	client.Plans = &plans{client: client}
	client.PlanExports = &planExports{client: client}
	client.Policies = &policies{client: client}
	client.PolicyChecks = &policyChecks{client: client}
	client.PolicySetParameters = &policySetParameters{client: client}
	client.PolicySetVersions = &policySetVersions{client: client}
	client.PolicySets = &policySets{client: client}
	client.RegistryModules = &registryModules{client: client}
	client.Runs = &runs{client: client}
	client.RunTasks = &runTasks{client: client}
	client.RunTriggers = &runTriggers{client: client}
	client.SSHKeys = &sshKeys{client: client}
	client.StateVersionOutputs = &stateVersionOutputs{client: client}
	client.StateVersions = &stateVersions{client: client}
	client.TaskStages = &taskStages{client: client}
	client.Teams = &teams{client: client}
	client.TeamAccess = &teamAccesses{client: client}
	client.TeamMembers = &teamMembers{client: client}
	client.TeamTokens = &teamTokens{client: client}
	client.Users = &users{client: client}
	client.UserTokens = &userTokens{client: client}
	client.Variables = &variables{client: client}
	client.VariableSets = &variableSets{client: client}
	client.VariableSetVariables = &variableSetVariables{client: client}
	client.Workspaces = &workspaces{client: client}
	client.WorkspaceRunTasks = &workspaceRunTasks{client: client}

	client.Meta = Meta{
		IPRanges: &ipRanges{client: client},
	}

	return client, nil
}

// RemoteAPIVersion returns the server's declared API version string.
//
// A Terraform Cloud or Enterprise API server returns its API version in an
// HTTP header field in all responses. The NewClient function saves the
// version number returned in its initial setup request and RemoteAPIVersion
// returns that cached value.
//
// The API protocol calls for this string to be a dotted-decimal version number
// like 2.3.0, where the first number indicates the API major version while the
// second indicates a minor version which may have introduced some
// backward-compatible additional features compared to its predecessor.
//
// Explicit API versioning was added to the Terraform Cloud and Enterprise
// APIs as a later addition, so older servers will not return version
// information. In that case, this function returns an empty string as the
// version.
func (c *Client) RemoteAPIVersion() string {
	return c.remoteAPIVersion
}

// SetFakeRemoteAPIVersion allows setting a given string as the client's remoteAPIVersion,
// overriding the value pulled from the API header during client initialization.
//
// This is intended for use in tests, when you may want to configure your TFE client to
// return something different than the actual API version in order to test error handling.

func (c *Client) SetFakeRemoteAPIVersion(fakeAPIVersion string) {
	c.remoteAPIVersion = fakeAPIVersion
}

// RetryServerErrors configures the retry HTTP check to also retry
// unexpected errors or requests that failed with a server error.

func (c *Client) RetryServerErrors(retry bool) {
	c.retryServerErrors = retry
}

// retryHTTPCheck provides a callback for Client.CheckRetry which
// will retry both rate limit (429) and server (>= 500) errors.

func (c *Client) retryHTTPCheck(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return c.retryServerErrors, err
	}
	if resp.StatusCode == 429 || (c.retryServerErrors && resp.StatusCode >= 500) {
		return true, nil
	}
	return false, nil
}

// retryHTTPBackoff provides a generic callback for Client.Backoff which
// will pass through all calls based on the status code of the response.

func (c *Client) retryHTTPBackoff(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
	if c.retryLogHook != nil {
		c.retryLogHook(attemptNum, resp)
	}

	// Use the rate limit backoff function when we are rate limited.
	if resp != nil && resp.StatusCode == 429 {
		return rateLimitBackoff(min, max, resp)
	}

	// Set custom duration's when we experience a service interruption.
	min = 700 * time.Millisecond
	max = 900 * time.Millisecond

	return retryablehttp.LinearJitterBackoff(min, max, attemptNum, resp)
}

// rateLimitBackoff provides a callback for Client.Backoff which will use the
// X-RateLimit_Reset header to determine the time to wait. We add some jitter
// to prevent a thundering herd.
//
// min and max are mainly used for bounding the jitter that will be added to
// the reset time retrieved from the headers. But if the final wait time is
// less then min, min will be used instead.

func rateLimitBackoff(min, max time.Duration, resp *http.Response) time.Duration {
	// rnd is used to generate pseudo-random numbers.
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	// First create some jitter bounded by the min and max durations.
	jitter := time.Duration(rnd.Float64() * float64(max-min))

	if resp != nil && resp.Header.Get(_headerRateReset) != "" {
		v := resp.Header.Get(_headerRateReset)
		reset, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Fatal(err)
		}
		// Only update min if the given time to wait is longer
		if reset > 0 && time.Duration(reset*1e9) > min {
			min = time.Duration(reset * 1e9)
		}
	}

	return min + jitter
}

type rawAPIMetadata struct {
	// APIVersion is the raw API version string reported by the server in the
	// TFP-API-Version response header, or an empty string if that header
	// field was not included in the response.
	APIVersion string

	// RateLimit is the raw API version string reported by the server in the
	// X-RateLimit-Limit response header, or an empty string if that header
	// field was not included in the response.
	RateLimit string
}

func (c *Client) getRawAPIMetadata() (rawAPIMetadata, error) {
	var meta rawAPIMetadata

	// Create a new request.
	u, err := c.baseURL.Parse(PingEndpoint)
	if err != nil {
		return meta, err
	}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return meta, err
	}

	// Attach the default headers.
	for k, v := range c.headers {
		req.Header[k] = v
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	// Make a single request to retrieve the rate limit headers.
	resp, err := c.http.HTTPClient.Do(req)
	if err != nil {
		return meta, err
	}
	resp.Body.Close()

	meta.APIVersion = resp.Header.Get(_headerAPIVersion)
	meta.RateLimit = resp.Header.Get(_headerRateLimit)

	return meta, nil
}

// configureLimiter configures the rate limiter.

func (c *Client) configureLimiter(rawLimit string) {
	// Set default values for when rate limiting is disabled.
	limit := rate.Inf
	burst := 0

	if v := rawLimit; v != "" {
		if rateLimit, err := strconv.ParseFloat(v, 64); rateLimit > 0 {
			if err != nil {
				log.Fatal(err)
			}
			// Configure the limit and burst using a split of 2/3 for the limit and
			// 1/3 for the burst. This enables clients to burst 1/3 of the allowed
			// calls before the limiter kicks in. The remaining calls will then be
			// spread out evenly using intervals of time.Second / limit which should
			// prevent hitting the rate limit.
			limit = rate.Limit(rateLimit * 0.66)
			burst = int(rateLimit * 0.33)
		}
	}

	// Create a new limiter using the calculated values.
	c.limiter = rate.NewLimiter(limit, burst)
}

// newRequest creates an API request with proper headers and serialization.
//
// A relative URL path can be provided, in which case it is resolved relative to the baseURL
// of the Client. Relative URL paths should always be specified without a preceding slash. Adding a
// preceding slash allows for ignoring the configured baseURL for non-standard endpoints.
//
// If v is supplied, the value will be JSONAPI encoded and included as the
// request body. If the method is GET, the value will be parsed and added as
// query parameters.
func (c *Client) newRequest(method, path string, v interface{}) (*retryablehttp.Request, error) {
	u, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, err
	}

	// Create a request specific headers map.
	reqHeaders := make(http.Header)
	reqHeaders.Set("Authorization", "Bearer "+c.token)

	var body interface{}
	switch method {
	case "GET":
		reqHeaders.Set("Accept", "application/vnd.api+json")

		if v != nil {
			q, err := query.Values(v)
			if err != nil {
				return nil, err
			}
			u.RawQuery = encodeQueryParams(q)
		}
	case "DELETE", "PATCH", "POST":
		reqHeaders.Set("Accept", "application/vnd.api+json")
		reqHeaders.Set("Content-Type", "application/vnd.api+json")

		if v != nil {
			if body, err = serializeRequestBody(v); err != nil {
				return nil, err
			}
		}
	case "PUT":
		reqHeaders.Set("Accept", "application/json")
		reqHeaders.Set("Content-Type", "application/octet-stream")
		body = v
	}

	req, err := retryablehttp.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}

	// Set the default headers.
	for k, v := range c.headers {
		req.Header[k] = v
	}

	// Set the request specific headers.
	for k, v := range reqHeaders {
		req.Header[k] = v
	}

	return req, nil
}

// Encode encodes the values into ``URL encoded'' form
// ("bar=baz&foo=quux") sorted by key.

func encodeQueryParams(v url.Values) string {
	if v == nil {
		return ""
	}
	var buf strings.Builder
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vs := v[k]
		if len(vs) > 1 && validSliceKey(k) {
			val := strings.Join(vs, ",")
			vs = vs[:0]
			vs = append(vs, val)
		}
		keyEscaped := url.QueryEscape(k)

		for _, v := range vs {
			if buf.Len() > 0 {
				buf.WriteByte('&')
			}
			buf.WriteString(keyEscaped)
			buf.WriteByte('=')
			buf.WriteString(url.QueryEscape(v))
		}
	}
	return buf.String()
}

// Helper method that serializes the given ptr or ptr slice into a JSON
// request. It automatically uses jsonapi or json serialization, depending
// on the body type's tags.

func serializeRequestBody(v interface{}) (interface{}, error) {
	// The body can be a slice of pointers or a pointer. In either
	// case we want to choose the serialization type based on the
	// individual record type. To determine that type, we need
	// to either follow the pointer or examine the slice element type.
	// There are other theoretical possibilities (e. g. maps,
	// non-pointers) but they wouldn't work anyway because the
	// json-api library doesn't support serializing other things.
	var modelType reflect.Type
	bodyType := reflect.TypeOf(v)
	switch bodyType.Kind() {
	case reflect.Slice:
		sliceElem := bodyType.Elem()
		if sliceElem.Kind() != reflect.Ptr {
			return nil, ErrInvalidRequestBody
		}
		modelType = sliceElem.Elem()
	case reflect.Ptr:
		modelType = reflect.ValueOf(v).Elem().Type()
	default:
		return nil, ErrInvalidRequestBody
	}

	// Infer whether the request uses jsonapi or regular json
	// serialization based on how the fields are tagged.
	jsonAPIFields := 0
	jsonFields := 0
	for i := 0; i < modelType.NumField(); i++ {
		structField := modelType.Field(i)
		if structField.Tag.Get("jsonapi") != "" {
			jsonAPIFields++
		}
		if structField.Tag.Get("json") != "" {
			jsonFields++
		}
	}
	if jsonAPIFields > 0 && jsonFields > 0 {
		// Defining a struct with both json and jsonapi tags doesn't
		// make sense, because a struct can only be serialized
		// as one or another. If this does happen, it's a bug
		// in the library that should be fixed at development time
		return nil, ErrInvalidStructFormat
	}

	if jsonFields > 0 {
		return json.Marshal(v)
	}
	buf := bytes.NewBuffer(nil)
	if err := jsonapi.MarshalPayloadWithoutIncluded(buf, v); err != nil {
		return nil, err
	}
	return buf, nil
}

// do sends an API request and returns the API response. The API response
// is JSONAPI decoded and the document's primary data is stored in the value
// pointed to by v, or returned as an error if an API error has occurred.

// If v implements the io.Writer interface, the raw response body will be
// written to v, without attempting to first decode it.
//
// The provided ctx must be non-nil. If it is canceled or times out, ctx.Err()
// will be returned.

func (c *Client) do(ctx context.Context, req *retryablehttp.Request, v interface{}) error {
	// Wait will block until the limiter can obtain a new token
	// or returns an error if the given context is canceled.
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}

	// Add the context to the request.
	reqWithCxt := req.WithContext(ctx)

	// Execute the request and check the response.
	resp, err := c.http.Do(reqWithCxt)
	if err != nil {
		// If we got an error, and the context has been canceled,
		// the context's error is probably more useful.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return err
		}
	}
	defer resp.Body.Close()

	// Basic response checking.
	if err := checkResponseCode(resp); err != nil {
		return err
	}

	// Return here if decoding the response isn't needed.
	if v == nil {
		return nil
	}

	// If v implements io.Writer, write the raw response body.
	if w, ok := v.(io.Writer); ok {
		_, err := io.Copy(w, resp.Body)
		return err
	}

	return unmarshalResponse(resp.Body, v)
}

// customDo is similar to func (c *Client) do(ctx context.Context, req *retryablehttp.Request, v interface{}) error. Except that The IP ranges API is not returning jsonapi like every other endpoint
// which means we need to handle it differently.

func (i *ipRanges) customDo(ctx context.Context, req *retryablehttp.Request, ir *IPRange) error {
	// Wait will block until the limiter can obtain a new token
	// or returns an error if the given context is canceled.
	if err := i.client.limiter.Wait(ctx); err != nil {
		return err
	}

	// Add the context to the request.
	req = req.WithContext(ctx)

	// Execute the request and check the response.
	resp, err := i.client.http.Do(req)
	if err != nil {
		// If we got an error, and the context has been canceled,
		// the context's error is probably more useful.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 && resp.StatusCode >= 400 {
		return fmt.Errorf("error HTTP response while retrieving IP ranges: %d", resp.StatusCode)
	} else if resp.StatusCode == 304 {
		return nil
	}

	err = json.NewDecoder(resp.Body).Decode(ir)
	if err != nil {
		return err
	}
	return nil
}

func unmarshalResponse(responseBody io.Reader, model interface{}) error {
	// Get the value of model so we can test if it's a struct.
	dst := reflect.Indirect(reflect.ValueOf(model))

	// Return an error if model is not a struct or an io.Writer.
	if dst.Kind() != reflect.Struct {
		return fmt.Errorf("%v must be a struct or an io.Writer", dst)
	}

	// Try to get the Items and Pagination struct fields.
	items := dst.FieldByName("Items")
	pagination := dst.FieldByName("Pagination")

	// Unmarshal a single value if model does not contain the
	// Items and Pagination struct fields.
	if !items.IsValid() || !pagination.IsValid() {
		return jsonapi.UnmarshalPayload(responseBody, model)
	}

	// Return an error if model.Items is not a slice.
	if items.Type().Kind() != reflect.Slice {
		return ErrItemsMustBeSlice
	}

	// Create a temporary buffer and copy all the read data into it.
	body := bytes.NewBuffer(nil)
	reader := io.TeeReader(responseBody, body)

	// Unmarshal as a list of values as model.Items is a slice.
	raw, err := jsonapi.UnmarshalManyPayload(reader, items.Type().Elem())
	if err != nil {
		return err
	}

	// Make a new slice to hold the results.
	sliceType := reflect.SliceOf(items.Type().Elem())
	result := reflect.MakeSlice(sliceType, 0, len(raw))

	// Add all of the results to the new slice.
	for _, v := range raw {
		result = reflect.Append(result, reflect.ValueOf(v))
	}

	// Pointer-swap the result.
	items.Set(result)

	// As we are getting a list of values, we need to decode
	// the pagination details out of the response body.
	p, err := parsePagination(body)
	if err != nil {
		return err
	}

	// Pointer-swap the decoded pagination details.
	pagination.Set(reflect.ValueOf(p))

	return nil
}

// ListOptions is used to specify pagination options when making API requests.
// Pagination allows breaking up large result sets into chunks, or "pages".
type ListOptions struct {
	// The page number to request. The results vary based on the PageSize.
	PageNumber int `url:"page[number],omitempty"`

	// The number of elements returned in a single page.
	PageSize int `url:"page[size],omitempty"`
}

// Pagination is used to return the pagination details of an API request.
type Pagination struct {
	CurrentPage  int `json:"current-page"`
	PreviousPage int `json:"prev-page"`
	NextPage     int `json:"next-page"`
	TotalPages   int `json:"total-pages"`
	TotalCount   int `json:"total-count"`
}

func parsePagination(body io.Reader) (*Pagination, error) {
	var raw struct {
		Meta struct {
			Pagination Pagination `jsonapi:"pagination"`
		} `jsonapi:"meta"`
	}

	// JSON decode the raw response.
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return &Pagination{}, err
	}

	return &raw.Meta.Pagination, nil
}

// checkResponseCode can be used to check the status code of an HTTP request.

func checkResponseCode(r *http.Response) error {
	if r.StatusCode >= 200 && r.StatusCode <= 299 {
		return nil
	}

	var errs []string
	var err error

	switch r.StatusCode {
	case 401:
		return ErrUnauthorized
	case 404:
		return ErrResourceNotFound
	case 409:
		switch {
		case strings.HasSuffix(r.Request.URL.Path, "actions/lock"):
			return ErrWorkspaceLocked
		case strings.HasSuffix(r.Request.URL.Path, "actions/unlock"):
			errs, err = decodeErrorPayload(r)
			if err != nil {
				return err
			}

			if errorPayloadContains(errs, "is locked by Run") {
				return ErrWorkspaceLockedByRun
			}

			return ErrWorkspaceNotLocked
		case strings.HasSuffix(r.Request.URL.Path, "actions/force-unlock"):
			return ErrWorkspaceNotLocked
		}
	}

	errs, err = decodeErrorPayload(r)
	if err != nil {
		return err
	}

	return fmt.Errorf(strings.Join(errs, "\n"))
}

func decodeErrorPayload(r *http.Response) ([]string, error) {
	// Decode the error payload.
	var errs []string
	errPayload := &jsonapi.ErrorsPayload{}
	err := json.NewDecoder(r.Body).Decode(errPayload)
	if err != nil || len(errPayload.Errors) == 0 {
		return errs, fmt.Errorf(r.Status)
	}

	// Parse and format the errors.
	for _, e := range errPayload.Errors {
		if e.Detail == "" {
			errs = append(errs, e.Title)
		} else {
			errs = append(errs, fmt.Sprintf("%s\n\n%s", e.Title, e.Detail))
		}
	}

	return errs, nil
}

func errorPayloadContains(payloadErrors []string, match string) bool {
	for _, e := range payloadErrors {
		if strings.Contains(e, match) {
			return true
		}
	}
	return false
}

func packContents(path string) (*bytes.Buffer, error) {
	body := bytes.NewBuffer(nil)

	file, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return body, fmt.Errorf(`failed to find files under the path "%v": %w`, path, err)
		}
		return body, fmt.Errorf(`unable to upload files from the path "%v": %w`, path, err)
	}

	if !file.Mode().IsDir() {
		return body, ErrMissingDirectory
	}

	_, errSlug := slug.Pack(path, body, true)
	if errSlug != nil {
		return body, errSlug
	}

	return body, nil
}

func validSliceKey(key string) bool {
	return key == _includeQueryParam || strings.Contains(key, "filter[")
}
