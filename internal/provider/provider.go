package provider

import (
	"context"
	"fmt"
	"net/http"

	motmedelGcp "github.com/Motmedel/utils_go/pkg/cloud/gcp"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/groups_settings"
	"github.com/Motmedel/utils_go/pkg/http/types/fetch_config"
	motmedelOauth2Transport "github.com/Motmedel/utils_go/pkg/oauth2/types/transport"
	"github.com/Motmedel/utils_go/pkg/utils"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &gwsProvider{}

type gwsProviderModel struct {
	ImpersonatedUserEmail types.String `tfsdk:"impersonated_user_email"`
}

type gwsProviderData struct {
	directoryClient      *directory.Client
	groupsSettingsClient *groups_settings.Client
	fetchOption          fetch_config.Option
}

type gwsProvider struct {
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &gwsProvider{
			version: version,
		}
	}
}

func (p *gwsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "gws"
	resp.Version = p.version
}

func (p *gwsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for Google Workspace.",
	}
}

func (p *gwsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config gwsProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gcpClient := motmedelGcp.NewClient()

	tokenSource, err := gcpClient.FindDefaultCredentials(
		context.Background(),
		[]string{
			"https://www.googleapis.com/auth/admin.directory.user",
			"https://www.googleapis.com/auth/admin.directory.group",
			"https://www.googleapis.com/auth/admin.directory.group.member",
			"https://www.googleapis.com/auth/apps.groups.settings",
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"An error occurred when finding default credentials.",
			fmt.Sprintf("Application Default Credentials not found: %s", err),
		)
		return
	}
	if utils.IsNil(tokenSource) {
		resp.Diagnostics.AddError(
			"The default credentials token source is nil.",
			"",
		)
		return
	}

	providerData := &gwsProviderData{
		directoryClient:      directory.NewClient(),
		groupsSettingsClient: groups_settings.NewClient(),
		fetchOption: fetch_config.WithHttpClient(
			&http.Client{
				Transport: &motmedelOauth2Transport.Transport{Source: tokenSource},
			},
		),
	}

	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func (p *gwsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewUserResource,
		NewGroupResource,
		NewGroupMemberResource,
		NewGroupSettingsResource,
	}
}

func (p *gwsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewGroupsDataSource,
	}
}

func getProviderData(providerData any, resp interface{ AddError(string, string) }) *gwsProviderData {
	if providerData == nil {
		return nil
	}

	data, ok := providerData.(*gwsProviderData)
	if !ok {
		resp.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *gwsProviderData, got %T", providerData),
		)
		return nil
	}

	return data
}

func fetchOptions(data *gwsProviderData) []fetch_config.Option {
	return []fetch_config.Option{data.fetchOption}
}
