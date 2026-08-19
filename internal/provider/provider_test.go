package provider

import (
	"slices"
	"testing"

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
