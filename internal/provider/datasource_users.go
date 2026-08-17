package provider

import (
	"context"
	"fmt"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/directory"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &usersDataSource{}

type usersDataSourceModel struct {
	Customer types.String     `tfsdk:"customer"`
	Users    []userEntryModel `tfsdk:"users"`
}

type userEntryModel struct {
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

type usersDataSource struct {
	client       *directory.Client
	providerData *gwsProviderData
}

func NewUsersDataSource() datasource.DataSource {
	return &usersDataSource{}
}

func (d *usersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *usersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all Google Workspace users.",
		Attributes: map[string]schema.Attribute{
			"customer": schema.StringAttribute{
				Optional:    true,
				Description: "Customer ID. Defaults to \"my_customer\".",
			},
			"users": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of users.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The unique ID for the user.",
						},
						"primary_email": schema.StringAttribute{
							Computed:    true,
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
				},
			},
		},
	}
}

func (d *usersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *usersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config usersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	customer := config.Customer.ValueString()
	if customer == "" {
		customer = "my_customer"
	}

	users, err := d.client.ListUsers(ctx, customer, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing users",
			fmt.Sprintf("Could not list users: %s", apiErrorDetail(err)),
		)
		return
	}

	var entries []userEntryModel
	for _, u := range users {
		entry := userEntryModel{
			Id:                         types.StringValue(u.Id),
			PrimaryEmail:               types.StringValue(u.PrimaryEmail),
			IsAdmin:                    types.BoolValue(u.IsAdmin),
			IsDelegatedAdmin:           types.BoolValue(u.IsDelegatedAdmin),
			AgreedToTerms:              types.BoolValue(u.AgreedToTerms),
			Suspended:                  types.BoolValue(u.Suspended),
			Archived:                   types.BoolValue(u.Archived),
			ChangePasswordAtNextLogin:  types.BoolValue(u.ChangePasswordAtNextLogin),
			IpWhitelisted:              types.BoolValue(u.IpWhitelisted),
			IsMailboxSetup:             types.BoolValue(u.IsMailboxSetup),
			IsEnrolledIn2Sv:            types.BoolValue(u.IsEnrolledIn2Sv),
			IsEnforcedIn2Sv:            types.BoolValue(u.IsEnforcedIn2Sv),
			IncludeInGlobalAddressList: types.BoolValue(u.IncludeInGlobalAddressList),
			CustomerId:                 types.StringValue(u.CustomerId),
			Etag:                       types.StringValue(u.Etag),
			OrgUnitPath:                types.StringValue(u.OrgUnitPath),
			RecoveryEmail:              types.StringValue(u.RecoveryEmail),
			RecoveryPhone:              types.StringValue(u.RecoveryPhone),
			SuspensionReason:           types.StringValue(u.SuspensionReason),
			CreationTime:               types.StringValue(u.CreationTime),
			LastLoginTime:              types.StringValue(u.LastLoginTime),
		}

		if u.Name != nil {
			entry.GivenName = types.StringValue(u.Name.GivenName)
			entry.FamilyName = types.StringValue(u.Name.FamilyName)
			entry.FullName = types.StringValue(u.Name.FullName)
		} else {
			entry.GivenName = types.StringNull()
			entry.FamilyName = types.StringNull()
			entry.FullName = types.StringNull()
		}

		entries = append(entries, entry)
	}

	config.Users = entries
	config.Customer = types.StringValue(customer)
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
