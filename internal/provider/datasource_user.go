package provider

import (
	"context"
	"fmt"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &userDataSource{}

type userDataSourceModel struct {
	Id                         types.String `tfsdk:"id"`
	PrimaryEmail               types.String `tfsdk:"primary_email"`
	GivenName                  types.String `tfsdk:"given_name"`
	FamilyName                 types.String `tfsdk:"family_name"`
	FullName                   types.String `tfsdk:"full_name"`
	IsAdmin                    types.Bool   `tfsdk:"is_admin"`
	IsDelegatedAdmin           types.Bool   `tfsdk:"is_delegated_admin"`
	AgreedToTerms              types.Bool   `tfsdk:"agreed_to_terms"`
	Suspended                  types.Bool   `tfsdk:"suspended"`
	Archived                   types.Bool   `tfsdk:"archived"`
	ChangePasswordAtNextLogin  types.Bool   `tfsdk:"change_password_at_next_login"`
	IpWhitelisted              types.Bool   `tfsdk:"ip_whitelisted"`
	IsMailboxSetup             types.Bool   `tfsdk:"is_mailbox_setup"`
	IsEnrolledIn2Sv            types.Bool   `tfsdk:"is_enrolled_in_2sv"`
	IsEnforcedIn2Sv            types.Bool   `tfsdk:"is_enforced_in_2sv"`
	IncludeInGlobalAddressList types.Bool   `tfsdk:"include_in_global_address_list"`
	CustomerId                 types.String `tfsdk:"customer_id"`
	Etag                       types.String `tfsdk:"etag"`
	OrgUnitPath                types.String `tfsdk:"org_unit_path"`
	RecoveryEmail              types.String `tfsdk:"recovery_email"`
	RecoveryPhone              types.String `tfsdk:"recovery_phone"`
	SuspensionReason           types.String `tfsdk:"suspension_reason"`
	CreationTime               types.String `tfsdk:"creation_time"`
	LastLoginTime              types.String `tfsdk:"last_login_time"`
}

type userDataSource struct {
	client       *directory.Client
	providerData *gwsProviderData
}

func NewUserDataSource() datasource.DataSource {
	return &userDataSource{}
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Google Workspace user.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique ID for the user.",
			},
			"primary_email": schema.StringAttribute{
				Required:    true,
				Description: "The user's primary email address.",
			},
			"given_name": schema.StringAttribute{
				Computed:    true,
				Description: "The user's first name.",
			},
			"family_name": schema.StringAttribute{
				Computed:    true,
				Description: "The user's last name.",
			},
			"full_name": schema.StringAttribute{
				Computed:    true,
				Description: "The user's full name.",
			},
			"is_admin": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user has super administrator privileges.",
			},
			"is_delegated_admin": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is a delegated administrator.",
			},
			"agreed_to_terms": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user has agreed to the terms of service.",
			},
			"suspended": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is suspended.",
			},
			"archived": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is archived.",
			},
			"change_password_at_next_login": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user must change their password at next login.",
			},
			"ip_whitelisted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user's IP address is whitelisted.",
			},
			"is_mailbox_setup": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user's Google mailbox is set up.",
			},
			"is_enrolled_in_2sv": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is enrolled in 2-step verification.",
			},
			"is_enforced_in_2sv": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether 2-step verification is enforced for the user.",
			},
			"include_in_global_address_list": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is included in the Global Address List.",
			},
			"customer_id": schema.StringAttribute{
				Computed:    true,
				Description: "The customer ID.",
			},
			"etag": schema.StringAttribute{
				Computed:    true,
				Description: "ETag of the resource.",
			},
			"org_unit_path": schema.StringAttribute{
				Computed:    true,
				Description: "The organizational unit path.",
			},
			"recovery_email": schema.StringAttribute{
				Computed:    true,
				Description: "The recovery email address.",
			},
			"recovery_phone": schema.StringAttribute{
				Computed:    true,
				Description: "The recovery phone number.",
			},
			"suspension_reason": schema.StringAttribute{
				Computed:    true,
				Description: "The reason the user account is suspended.",
			},
			"creation_time": schema.StringAttribute{
				Computed:    true,
				Description: "The time the user account was created.",
			},
			"last_login_time": schema.StringAttribute{
				Computed:    true,
				Description: "The last time the user logged in.",
			},
		},
	}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*gwsProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *gwsProviderData, got %T", req.ProviderData),
		)
		return
	}

	d.client = data.directoryClient
	d.providerData = data
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userKey := config.PrimaryEmail.ValueString()

	apiUser, err := d.client.GetUser(ctx, userKey, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading user",
			fmt.Sprintf("Could not read user %s: %s", userKey, apiErrorDetail(err)),
		)
		return
	}

	if apiUser == nil {
		resp.Diagnostics.AddError(
			"User not found",
			fmt.Sprintf("User %s not found.", userKey),
		)
		return
	}

	config.Id = types.StringValue(apiUser.Id)
	config.PrimaryEmail = types.StringValue(apiUser.PrimaryEmail)
	config.CustomerId = types.StringValue(apiUser.CustomerId)
	config.Etag = types.StringValue(apiUser.Etag)
	config.IsAdmin = types.BoolValue(apiUser.IsAdmin)
	config.IsDelegatedAdmin = types.BoolValue(apiUser.IsDelegatedAdmin)
	config.AgreedToTerms = types.BoolValue(apiUser.AgreedToTerms)
	config.Suspended = types.BoolValue(apiUser.Suspended)
	config.Archived = types.BoolValue(apiUser.Archived)
	config.IsMailboxSetup = types.BoolValue(apiUser.IsMailboxSetup)
	config.ChangePasswordAtNextLogin = types.BoolValue(apiUser.ChangePasswordAtNextLogin)
	config.IpWhitelisted = types.BoolValue(apiUser.IpWhitelisted)
	config.IsEnrolledIn2Sv = types.BoolValue(apiUser.IsEnrolledIn2Sv)
	config.IsEnforcedIn2Sv = types.BoolValue(apiUser.IsEnforcedIn2Sv)
	config.IncludeInGlobalAddressList = types.BoolValue(apiUser.IncludeInGlobalAddressList)
	config.SuspensionReason = types.StringValue(apiUser.SuspensionReason)
	config.OrgUnitPath = types.StringValue(apiUser.OrgUnitPath)
	config.RecoveryEmail = types.StringValue(apiUser.RecoveryEmail)
	config.RecoveryPhone = types.StringValue(apiUser.RecoveryPhone)
	config.LastLoginTime = types.StringValue(apiUser.LastLoginTime)
	config.CreationTime = types.StringValue(apiUser.CreationTime)

	if apiUser.Name != nil {
		config.GivenName = types.StringValue(apiUser.Name.GivenName)
		config.FamilyName = types.StringValue(apiUser.Name.FamilyName)
		config.FullName = types.StringValue(apiUser.Name.FullName)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
