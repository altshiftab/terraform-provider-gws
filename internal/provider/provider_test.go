package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json/v2"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/altshiftab/utils_go/pkg/http/types/fetch_config"
	"github.com/altshiftab/utils_go/pkg/oauth2/types/token"
	"github.com/altshiftab/utils_go/pkg/oauth2/types/token_source"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestResolveScopes(t *testing.T) {
	t.Parallel()

	readonlyUser := "https://www.googleapis.com/auth/admin.directory.user.readonly"

	testCases := []struct {
		name       string
		configured types.List
		want       []string
		wantErr    bool
	}{
		{
			name: "the full set is asked for as given",
			configured: types.ListValueMust(
				types.StringType,
				[]attr.Value{types.StringValue(defaultScopes[0])},
			),
			want: []string{defaultScopes[0]},
		},
		{
			name:       "unset is refused rather than defaulted",
			configured: types.ListNull(types.StringType),
			wantErr:    true,
		},
		{
			name:       "unknown is refused",
			configured: types.ListUnknown(types.StringType),
			wantErr:    true,
		},
		{
			name: "a narrower set is asked for as given",
			configured: types.ListValueMust(
				types.StringType,
				[]attr.Value{types.StringValue(readonlyUser)},
			),
			want: []string{readonlyUser},
		},
		{
			name:       "an empty list is refused rather than widened",
			configured: types.ListValueMust(types.StringType, []attr.Value{}),
			wantErr:    true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			scopes, diagnostics := resolveScopes(t.Context(), testCase.configured)

			if testCase.wantErr {
				if !diagnostics.HasError() {
					t.Error("expected an error")
				}
				return
			}
			if diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
			if !slices.Equal(scopes, testCase.want) {
				t.Errorf("unexpected scopes: got %v, want %v", scopes, testCase.want)
			}
		})
	}
}

var errNoDefaultCredentials = errors.New("no default credentials")

func TestImpersonationSigner(t *testing.T) {
	t.Parallel()

	defaultCredentials := func(source token_source.TokenSource, err error) func(context.Context, []string, ...fetch_config.Option) (token_source.TokenSource, error) {
		return func(context.Context, []string, ...fetch_config.Option) (token_source.TokenSource, error) {
			return source, err
		}
	}
	adcSource := token_source.NewStatic(&token.Token{AccessToken: "from-adc", TokenType: "Bearer"})

	testCases := []struct {
		name                   string
		accessToken            string
		findDefaultCredentials func(context.Context, []string, ...fetch_config.Option) (token_source.TokenSource, error)
		wantHeader             string
		wantErr                error
	}{
		{ //nolint:gosec // a placeholder, not a credential
			name:                   "a given token signs in place of ADC",
			accessToken:            "minted-elsewhere",
			findDefaultCredentials: defaultCredentials(adcSource, nil),
			wantHeader:             "Bearer minted-elsewhere",
		},
		{
			name:                   "a given token needs no ADC at all",
			accessToken:            "minted-elsewhere",
			findDefaultCredentials: defaultCredentials(nil, errNoDefaultCredentials),
			wantHeader:             "Bearer minted-elsewhere",
		},
		{
			name:                   "without a token, ADC signs",
			findDefaultCredentials: defaultCredentials(adcSource, nil),
			wantHeader:             "Bearer from-adc",
		},
		{
			name:                   "without a token or ADC, the failure is reported",
			findDefaultCredentials: defaultCredentials(nil, errNoDefaultCredentials),
			wantErr:                errNoDefaultCredentials,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			signer, err := impersonationSigner(t.Context(), testCase.accessToken, testCase.findDefaultCredentials)

			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) {
					t.Errorf("unexpected error: got %v, want %v", err, testCase.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if signer == nil {
				t.Fatal("expected a signer")
			}

			signerToken, err := signer.Token()
			if err != nil {
				t.Fatalf("unexpected token error: %v", err)
			}
			if signerToken == nil {
				t.Fatal("expected a token")
			}

			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://iamcredentials.googleapis.com/", nil)
			signerToken.SetAuthHeader(request)
			if header := request.Header.Get("Authorization"); header != testCase.wantHeader {
				t.Errorf("unexpected Authorization header: got %q, want %q", header, testCase.wantHeader)
			}
		})
	}
}

// TestConfigure_TokenSourceOutlivesConfigure is the refresh that failed cloudsqlpg's provider on a real apply:
// Terraform cancels Configure's context when Configure returns, and the clients fetch their first token later.
func TestConfigure_TokenSourceOutlivesConfigure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"fetched-later","expires_in":3600,"token_type":"Bearer"}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer fetched-later" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyDer, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	credentials, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"client_email":   "tf@proj.iam.gserviceaccount.com",
		"private_key_id": "key-1",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDer})),
		"token_uri":      server.URL + "/token",
	})
	if err != nil {
		t.Fatalf("marshal credentials: %v", err)
	}
	credentialsPath := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(credentialsPath, credentials, 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credentialsPath)
	t.Setenv(accessTokenEnvironmentVariable, "")

	// An empty configuration: no oauth2, service_account or impersonation block, so Application Default Credentials.
	ctx, cancel := context.WithCancel(context.Background())
	gws := New("test")()
	schemaResp := &provider.SchemaResponse{}
	gws.Schema(ctx, provider.SchemaRequest{}, schemaResp)
	objectType, ok := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("provider schema type is %T, want tftypes.Object", schemaResp.Schema.Type().TerraformType(ctx))
	}
	nulls := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		nulls[name] = tftypes.NewValue(attributeType, nil)
	}
	configureResp := &provider.ConfigureResponse{}
	gws.Configure(
		ctx,
		provider.ConfigureRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nulls)}},
		configureResp,
	)
	cancel()
	if configureResp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", configureResp.Diagnostics)
	}

	data, ok := configureResp.ResourceData.(*gwsProviderData)
	if !ok || data == nil {
		t.Fatalf("resource data is %T, want *gwsProviderData", configureResp.ResourceData)
	}
	client := fetch_config.New(data.fetchOption).HttpClient
	if client == nil {
		t.Fatal("the fetch option carries no HTTP client")
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/api", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request after Configure's context ended: %v", err)
	}
	if response == nil {
		t.Fatal("request after Configure's context ended: no response")
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d: the request did not carry the token fetched after Configure", response.StatusCode, http.StatusNoContent)
	}
}
