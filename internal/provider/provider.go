package provider

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"

	motmedelGcp "github.com/Motmedel/utils_go/pkg/cloud/gcp"
	"github.com/Motmedel/utils_go/pkg/cloud/gcp/types/credentials_file"
	domainWideDelegationTokenSource "github.com/Motmedel/utils_go/pkg/cloud/gcp/types/token_source/domain_wide_delegation_token_source"
	serviceAccountTokenSource "github.com/Motmedel/utils_go/pkg/cloud/gcp/types/token_source/service_account_token_source"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/drive"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/drive/drive_config"
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

type gwsProviderServiceAccountModel struct {
	Credentials types.String `tfsdk:"credentials"`
	Subject     types.String `tfsdk:"subject"`
}

type gwsProviderImpersonationModel struct {
	ServiceAccount types.String `tfsdk:"service_account"`
	Subject        types.String `tfsdk:"subject"`
}

type gwsProviderModel struct {
	Oauth2         *gwsProviderOauth2Model         `tfsdk:"oauth2"`
	ServiceAccount *gwsProviderServiceAccountModel `tfsdk:"service_account"`
	Impersonation  *gwsProviderImpersonationModel  `tfsdk:"impersonation"`
}

type gwsProviderData struct {
	directoryClient      *directory.Client
	groupsSettingsClient *groups_settings.Client
	gmailClient          *gmail.Client
	driveClient          *drive.Client
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
			"service_account": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Service account credentials with Google Workspace domain-wide delegation; the service account impersonates `subject`. Required for Gmail send-as, which is only available to DWD service account clients. Mutually exclusive with `oauth2`.",
				Attributes: map[string]schema.Attribute{
					"credentials": schema.StringAttribute{
						Required:    true,
						Sensitive:   true,
						Description: "Service account key JSON.",
					},
					"subject": schema.StringAttribute{
						Required:    true,
						Description: "Email address of the user to impersonate (the mailbox to act on).",
					},
				},
			},
			"impersonation": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Keyless Google Workspace domain-wide delegation. The provider's Application Default Credentials impersonate `service_account` (via IAM signJwt) to act as `subject` — no key is used. The ADC identity needs roles/iam.serviceAccountTokenCreator on the service account, and iamcredentials.googleapis.com must be enabled. Mutually exclusive with `oauth2` and `service_account`.",
				Attributes: map[string]schema.Attribute{
					"service_account": schema.StringAttribute{
						Required:    true,
						Description: "Email of the service account that has domain-wide delegation.",
					},
					"subject": schema.StringAttribute{
						Required:    true,
						Description: "Email address of the user to impersonate (the mailbox to act on).",
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

	authModes := 0
	if config.Oauth2 != nil {
		authModes++
	}
	if config.ServiceAccount != nil {
		authModes++
	}
	if config.Impersonation != nil {
		authModes++
	}
	if authModes > 1 {
		resp.Diagnostics.AddError(
			"Conflicting authentication configuration",
			"At most one of `oauth2`, `service_account`, or `impersonation` may be set.",
		)
		return
	}

	var tokenSource token_source.TokenSource
	switch {
	case config.Oauth2 != nil:
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
	case config.ServiceAccount != nil:
		var credentialsFile credentials_file.File
		if err := json.Unmarshal([]byte(config.ServiceAccount.Credentials.ValueString()), &credentialsFile); err != nil {
			resp.Diagnostics.AddError(
				"Invalid service account credentials",
				fmt.Sprintf("Could not parse the service account key JSON: %s", err),
			)
			return
		}
		ts, err := serviceAccountTokenSource.NewFromCredentialsFileWithSubject(
			context.Background(),
			googleTokenURL,
			&credentialsFile,
			[]string{
				"https://www.googleapis.com/auth/admin.directory.user",
				"https://www.googleapis.com/auth/admin.directory.group",
				"https://www.googleapis.com/auth/admin.directory.group.member",
				"https://www.googleapis.com/auth/apps.groups.settings",
				"https://www.googleapis.com/auth/gmail.settings.basic",
				"https://www.googleapis.com/auth/gmail.settings.sharing",
				drive.ScopeDrive,
			},
			config.ServiceAccount.Subject.ValueString(),
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"An error occurred when building the service account token source.",
				fmt.Sprintf("Could not build service account token source: %s", apiErrorDetail(err)),
			)
			return
		}
		if utils.IsNil(ts) {
			resp.Diagnostics.AddError(
				"The service account token source is nil.",
				"",
			)
			return
		}
		tokenSource = token_source.NewReusable(nil, ts)
	case config.Impersonation != nil:
		gcpClient := motmedelGcp.NewClient()
		signer, err := gcpClient.FindDefaultCredentials(
			context.Background(),
			[]string{"https://www.googleapis.com/auth/cloud-platform"},
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"An error occurred when finding default credentials for impersonation.",
				fmt.Sprintf("Application Default Credentials not found: %s", apiErrorDetail(err)),
			)
			return
		}
		if utils.IsNil(signer) {
			resp.Diagnostics.AddError(
				"The default credentials token source is nil.",
				"",
			)
			return
		}
		ts, err := domainWideDelegationTokenSource.New(
			context.Background(),
			signer,
			config.Impersonation.ServiceAccount.ValueString(),
			config.Impersonation.Subject.ValueString(),
			[]string{
				"https://www.googleapis.com/auth/admin.directory.user",
				"https://www.googleapis.com/auth/admin.directory.group",
				"https://www.googleapis.com/auth/admin.directory.group.member",
				"https://www.googleapis.com/auth/apps.groups.settings",
				"https://www.googleapis.com/auth/gmail.settings.basic",
				"https://www.googleapis.com/auth/gmail.settings.sharing",
				drive.ScopeDrive,
			},
			googleTokenURL,
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"An error occurred when building the impersonation token source.",
				fmt.Sprintf("Could not build impersonation token source: %s", apiErrorDetail(err)),
			)
			return
		}
		tokenSource = ts
	default:
		gcpClient := motmedelGcp.NewClient()
		ts, err := gcpClient.FindDefaultCredentials(
			context.Background(),
			[]string{
				"https://www.googleapis.com/auth/admin.directory.user",
				"https://www.googleapis.com/auth/admin.directory.group",
				"https://www.googleapis.com/auth/admin.directory.group.member",
				"https://www.googleapis.com/auth/apps.groups.settings",
				"https://www.googleapis.com/auth/gmail.settings.basic",
				drive.ScopeDrive,
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
		driveClient:          drive.NewClient(drive_config.WithSupportsAllDrives(true)),
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
		NewDrivePermissionResource,
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
		NewDrivePermissionDataSource,
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
