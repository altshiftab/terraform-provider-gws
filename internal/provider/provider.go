package provider

import (
	"context"
	"fmt"
	"net/http"

	motmedelGcp "github.com/Motmedel/utils_go/pkg/cloud/gcp"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/gmail"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/groups_settings"
	"github.com/Motmedel/utils_go/pkg/http/types/fetch_config"
	oauth2Config "github.com/Motmedel/utils_go/pkg/oauth2/types/config"
	"github.com/Motmedel/utils_go/pkg/oauth2/types/endpoint"
	"github.com/Motmedel/utils_go/pkg/oauth2/types/token"
	"github.com/Motmedel/utils_go/pkg/oauth2/types/token_source"
	motmedelOauth2Transport "github.com/Motmedel/utils_go/pkg/oauth2/types/transport"
	"github.com/Motmedel/utils_go/pkg/utils"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const googleTokenURL = "https://oauth2.googleapis.com/token"

var _ provider.Provider = &gwsProvider{}

type gwsProviderOauth2Model struct {
	ClientId     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	RefreshToken types.String `tfsdk:"refresh_token"`
	TokenUrl     types.String `tfsdk:"token_url"`
}

type gwsProviderModel struct {
	Oauth2 *gwsProviderOauth2Model `tfsdk:"oauth2"`
}

type gwsProviderData struct {
	directoryClient      *directory.Client
	groupsSettingsClient *groups_settings.Client
	gmailClient          *gmail.Client
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
		Attributes: map[string]schema.Attribute{
			"oauth2": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "OAuth2 credentials for authenticating as a specific user via a refresh token. When omitted, Application Default Credentials are used.",
				Attributes: map[string]schema.Attribute{
					"client_id": schema.StringAttribute{
						Required:    true,
						Description: "OAuth2 client ID.",
					},
					"client_secret": schema.StringAttribute{
						Required:    true,
						Sensitive:   true,
						Description: "OAuth2 client secret.",
					},
					"refresh_token": schema.StringAttribute{
						Required:    true,
						Sensitive:   true,
						Description: "Refresh token granted to the OAuth2 client for the target user.",
					},
					"token_url": schema.StringAttribute{
						Optional:    true,
						Description: "Token endpoint URL. Defaults to Google's endpoint.",
					},
				},
			},
		},
	}
}

func (p *gwsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config gwsProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var tokenSource token_source.TokenSource
	if config.Oauth2 != nil {
		tokenUrl := config.Oauth2.TokenUrl.ValueString()
		if tokenUrl == "" {
			tokenUrl = googleTokenURL
		}
		cfg := &oauth2Config.Config{
			ClientID:     config.Oauth2.ClientId.ValueString(),
			ClientSecret: config.Oauth2.ClientSecret.ValueString(),
			Endpoint: endpoint.Endpoint{
				TokenURL:  tokenUrl,
				AuthStyle: endpoint.AuthStyleInParams,
			},
		}
		tokenSource = cfg.TokenSource(
			context.Background(),
			&token.Token{RefreshToken: config.Oauth2.RefreshToken.ValueString()},
		)
	} else {
		gcpClient := motmedelGcp.NewClient()
		ts, err := gcpClient.FindDefaultCredentials(
			context.Background(),
			[]string{
				"https://www.googleapis.com/auth/admin.directory.user",
				"https://www.googleapis.com/auth/admin.directory.group",
				"https://www.googleapis.com/auth/admin.directory.group.member",
				"https://www.googleapis.com/auth/apps.groups.settings",
				"https://www.googleapis.com/auth/gmail.settings.basic",
			},
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"An error occurred when finding default credentials.",
				fmt.Sprintf("Application Default Credentials not found: %s", apiErrorDetail(err)),
			)
			return
		}
		if utils.IsNil(ts) {
			resp.Diagnostics.AddError(
				"The default credentials token source is nil.",
				"",
			)
			return
		}
		tokenSource = ts
	}

	providerData := &gwsProviderData{
		directoryClient:      directory.NewClient(),
		groupsSettingsClient: groups_settings.NewClient(),
		gmailClient:          gmail.NewClient(),
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
		NewGmailSendAsResource,
		NewGmailFilterResource,
	}
}

func (p *gwsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewUserDataSource,
		NewUsersDataSource,
		NewGroupDataSource,
		NewGroupsDataSource,
		NewGroupMemberDataSource,
		NewGroupMembersDataSource,
		NewGroupSettingsDataSource,
		NewGmailSendAsDataSource,
		NewGmailFilterDataSource,
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
