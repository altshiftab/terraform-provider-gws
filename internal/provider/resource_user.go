package provider

import (
	"context"
	"fmt"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory/types/user"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory/types/user/name"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &userResource{}
	_ resource.ResourceWithImportState = &userResource{}
)

type userResourceModel struct {
	Id                        types.String `tfsdk:"id"`
	PrimaryEmail              types.String `tfsdk:"primary_email"`
	GivenName                 types.String `tfsdk:"given_name"`
	FamilyName                types.String `tfsdk:"family_name"`
	Password                  types.String `tfsdk:"password"`
	HashFunction              types.String `tfsdk:"hash_function"`
	IsAdmin                   types.Bool   `tfsdk:"is_admin"`
	Suspended                 types.Bool   `tfsdk:"suspended"`
	Archived                  types.Bool   `tfsdk:"archived"`
	ChangePasswordAtNextLogin types.Bool   `tfsdk:"change_password_at_next_login"`
	IpWhitelisted             types.Bool   `tfsdk:"ip_whitelisted"`
	IncludeInGlobalAddressList types.Bool  `tfsdk:"include_in_global_address_list"`
	OrgUnitPath               types.String `tfsdk:"org_unit_path"`
	RecoveryEmail             types.String `tfsdk:"recovery_email"`
	RecoveryPhone             types.String `tfsdk:"recovery_phone"`

	// Computed
	CustomerId       types.String `tfsdk:"customer_id"`
	Etag             types.String `tfsdk:"etag"`
	FullName         types.String `tfsdk:"full_name"`
	IsMailboxSetup   types.Bool   `tfsdk:"is_mailbox_setup"`
	IsDelegatedAdmin types.Bool   `tfsdk:"is_delegated_admin"`
	AgreedToTerms    types.Bool   `tfsdk:"agreed_to_terms"`
	IsEnrolledIn2Sv  types.Bool   `tfsdk:"is_enrolled_in_2sv"`
	IsEnforcedIn2Sv  types.Bool   `tfsdk:"is_enforced_in_2sv"`
	SuspensionReason types.String `tfsdk:"suspension_reason"`
	CreationTime     types.String `tfsdk:"creation_time"`
	LastLoginTime    types.String `tfsdk:"last_login_time"`
}

type userResource struct {
	client      *directory.Client
	providerData *gwsProviderData
}

func NewUserResource() resource.Resource {
	return &userResource{}
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Google Workspace user.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique ID for the user.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"primary_email": schema.StringAttribute{
				Required:    true,
				Description: "The user's primary email address.",
			},
			"given_name": schema.StringAttribute{
				Required:    true,
				Description: "The user's first name.",
			},
			"family_name": schema.StringAttribute{
				Required:    true,
				Description: "The user's last name.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "The user's password. Required when creating a new user.",
			},
			"hash_function": schema.StringAttribute{
				Optional:    true,
				Description: "The hash function used to store the password. Supported values: MD5, SHA-1, crypt.",
			},
			"is_admin": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the user has super administrator privileges.",
			},
			"suspended": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the user is suspended.",
			},
			"archived": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the user is archived.",
			},
			"change_password_at_next_login": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the user must change their password at next login.",
			},
			"ip_whitelisted": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the user's IP address is allowlisted.",
			},
			"include_in_global_address_list": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the user's profile is visible in the Global Address List.",
			},
			"org_unit_path": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The organizational unit path for the user.",
			},
			"recovery_email": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The user's recovery email address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"recovery_phone": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The user's recovery phone number.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"customer_id": schema.StringAttribute{
				Computed:    true,
				Description: "The customer ID to which the user belongs.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"etag": schema.StringAttribute{
				Computed:    true,
				Description: "ETag of the resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"full_name": schema.StringAttribute{
				Computed:    true,
				Description: "The user's full name formed by concatenating first and last name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"is_mailbox_setup": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user's Google mailbox has been created.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_delegated_admin": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is a delegated administrator.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"agreed_to_terms": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user has agreed to the terms of service.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_enrolled_in_2sv": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is enrolled in 2-step verification.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_enforced_in_2sv": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether 2-step verification is enforced for the user.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"suspension_reason": schema.StringAttribute{
				Computed:    true,
				Description: "The reason the user account is suspended.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"creation_time": schema.StringAttribute{
				Computed:    true,
				Description: "The time the user account was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"last_login_time": schema.StringAttribute{
				Computed:    true,
				Description: "The last time the user logged in.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := getProviderData(req.ProviderData, &resp.Diagnostics)
	if data == nil {
		return
	}
	r.client = data.directoryClient
	r.providerData = data
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiUser := &user.User{
		PrimaryEmail: plan.PrimaryEmail.ValueString(),
		Name: &name.Name{
			GivenName:  plan.GivenName.ValueString(),
			FamilyName: plan.FamilyName.ValueString(),
		},
		Password:                   plan.Password.ValueString(),
		HashFunction:               plan.HashFunction.ValueString(),
		IsAdmin:                    plan.IsAdmin.ValueBool(),
		Suspended:                  plan.Suspended.ValueBool(),
		Archived:                   plan.Archived.ValueBool(),
		ChangePasswordAtNextLogin:  plan.ChangePasswordAtNextLogin.ValueBool(),
		IpWhitelisted:              plan.IpWhitelisted.ValueBool(),
		IncludeInGlobalAddressList: plan.IncludeInGlobalAddressList.ValueBool(),
		OrgUnitPath:                plan.OrgUnitPath.ValueString(),
		RecoveryEmail:              plan.RecoveryEmail.ValueString(),
		RecoveryPhone:              plan.RecoveryPhone.ValueString(),
	}

	createdUser, err := r.client.CreateUser(ctx, apiUser, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating user",
			fmt.Sprintf("Could not create user %s: %s", plan.PrimaryEmail.ValueString(), apiErrorDetail(err)),
		)
		return
	}

	mapUserToState(createdUser, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userKey := state.PrimaryEmail.ValueString()
	if userKey == "" {
		userKey = state.Id.ValueString()
	}

	apiUser, err := r.client.GetUser(ctx, userKey, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading user",
			fmt.Sprintf("Could not read user %s: %s", userKey, apiErrorDetail(err)),
		)
		return
	}

	if apiUser == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapUserToState(apiUser, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userResourceModel
	var state userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userKey := state.PrimaryEmail.ValueString()
	if userKey == "" {
		userKey = state.Id.ValueString()
	}

	apiUser := &user.User{
		PrimaryEmail: plan.PrimaryEmail.ValueString(),
		Name: &name.Name{
			GivenName:  plan.GivenName.ValueString(),
			FamilyName: plan.FamilyName.ValueString(),
		},
		IsAdmin:                    plan.IsAdmin.ValueBool(),
		Suspended:                  plan.Suspended.ValueBool(),
		Archived:                   plan.Archived.ValueBool(),
		ChangePasswordAtNextLogin:  plan.ChangePasswordAtNextLogin.ValueBool(),
		IpWhitelisted:              plan.IpWhitelisted.ValueBool(),
		IncludeInGlobalAddressList: plan.IncludeInGlobalAddressList.ValueBool(),
		OrgUnitPath:                plan.OrgUnitPath.ValueString(),
		RecoveryEmail:              plan.RecoveryEmail.ValueString(),
		RecoveryPhone:              plan.RecoveryPhone.ValueString(),
	}

	if !plan.Password.IsNull() && !plan.Password.IsUnknown() {
		apiUser.Password = plan.Password.ValueString()
		apiUser.HashFunction = plan.HashFunction.ValueString()
	}

	updatedUser, err := r.client.UpdateUser(ctx, userKey, apiUser, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating user",
			fmt.Sprintf("Could not update user %s: %s", userKey, apiErrorDetail(err)),
		)
		return
	}

	mapUserToState(updatedUser, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userKey := state.Id.ValueString()
	if userKey == "" {
		userKey = state.PrimaryEmail.ValueString()
	}

	err := r.client.DeleteUser(ctx, userKey, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting user",
			fmt.Sprintf("Could not delete user %s: %s", userKey, apiErrorDetail(err)),
		)
		return
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("primary_email"), req, resp)
}

func mapUserToState(u *user.User, state *userResourceModel) {
	if u == nil {
		return
	}

	state.Id = types.StringValue(u.Id)
	state.PrimaryEmail = types.StringValue(u.PrimaryEmail)
	state.CustomerId = types.StringValue(u.CustomerId)
	state.Etag = types.StringValue(u.Etag)
	state.IsAdmin = types.BoolValue(u.IsAdmin)
	state.IsDelegatedAdmin = types.BoolValue(u.IsDelegatedAdmin)
	state.AgreedToTerms = types.BoolValue(u.AgreedToTerms)
	state.Suspended = types.BoolValue(u.Suspended)
	state.Archived = types.BoolValue(u.Archived)
	state.IsMailboxSetup = types.BoolValue(u.IsMailboxSetup)
	state.ChangePasswordAtNextLogin = types.BoolValue(u.ChangePasswordAtNextLogin)
	state.IpWhitelisted = types.BoolValue(u.IpWhitelisted)
	state.IsEnrolledIn2Sv = types.BoolValue(u.IsEnrolledIn2Sv)
	state.IsEnforcedIn2Sv = types.BoolValue(u.IsEnforcedIn2Sv)
	state.IncludeInGlobalAddressList = types.BoolValue(u.IncludeInGlobalAddressList)
	state.SuspensionReason = types.StringValue(u.SuspensionReason)
	state.OrgUnitPath = types.StringValue(u.OrgUnitPath)
	state.RecoveryEmail = types.StringValue(u.RecoveryEmail)
	state.RecoveryPhone = types.StringValue(u.RecoveryPhone)
	state.LastLoginTime = types.StringValue(u.LastLoginTime)
	state.CreationTime = types.StringValue(u.CreationTime)

	if u.Name != nil {
		state.GivenName = types.StringValue(u.Name.GivenName)
		state.FamilyName = types.StringValue(u.Name.FamilyName)
		state.FullName = types.StringValue(u.Name.FullName)
	}

	// Password is write-only; preserve the existing state value.
}
