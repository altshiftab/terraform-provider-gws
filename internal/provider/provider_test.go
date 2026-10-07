package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/altshiftab/utils_go/pkg/http/types/fetch_config"
	"github.com/altshiftab/utils_go/pkg/oauth2/types/token"
	"github.com/altshiftab/utils_go/pkg/oauth2/types/token_source"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
