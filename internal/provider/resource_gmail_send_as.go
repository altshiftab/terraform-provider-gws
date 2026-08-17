package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/gmail"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/gmail/types/send_as"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &gmailSendAsResource{}
	_ resource.ResourceWithImportState = &gmailSendAsResource{}
)

type gmailSendAsResourceModel struct {
	UserId             types.String `tfsdk:"user_id"`
	SendAsEmail        types.String `tfsdk:"send_as_email"`
	DisplayName        types.String `tfsdk:"display_name"`
	ReplyToAddress     types.String `tfsdk:"reply_to_address"`
	Signature          types.String `tfsdk:"signature"`
	TreatAsAlias       types.Bool   `tfsdk:"treat_as_alias"`
	IsPrimary          types.Bool   `tfsdk:"is_primary"`
	IsDefault          types.Bool   `tfsdk:"is_default"`
	VerificationStatus types.String `tfsdk:"verification_status"`
}

type gmailSendAsResource struct {
	client       *gmail.Client
	providerData *gwsProviderData
}

func NewGmailSendAsResource() resource.Resource {
	return &gmailSendAsResource{}
}

func (r *gmailSendAsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gmail_send_as"
}

func (r *gmailSendAsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Gmail send-as alias for a Google Workspace user.",
		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "The user's email address or the special value \"me\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"send_as_email": schema.StringAttribute{
				Required:    true,
				Description: "The email address that appears in the \"From:\" header for mail sent using this alias.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				Optional:    true,
				Description: "A name that appears in the \"From:\" header for mail sent using this alias.",
			},
			"reply_to_address": schema.StringAttribute{
				Optional:    true,
				Description: "An optional email address that is included in a \"Reply-To:\" header for mail sent using this alias.",
			},
			"signature": schema.StringAttribute{
				Optional:    true,
				Description: "An optional HTML signature that is included in messages composed with this alias in the Gmail web UI.",
			},
			"treat_as_alias": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether Gmail should treat this address as an alias for the user's primary email address.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_primary": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this address is the primary address used to login to the account.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_default": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this address is selected as the default \"From:\" address in situations such as composing a new message or sending a vacation auto-reply.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"verification_status": schema.StringAttribute{
				Computed:    true,
				Description: "Indicates whether this address has been verified for use as a send-as alias.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *gmailSendAsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := getProviderData(req.ProviderData, &resp.Diagnostics)
	if data == nil {
		return
	}
	r.client = data.gmailClient
	r.providerData = data
}

func (r *gmailSendAsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gmailSendAsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiSendAs := buildSendAsFromPlan(&plan)

	userId := plan.UserId.ValueString()
	created, err := r.client.CreateSendAs(ctx, userId, apiSendAs, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Gmail send-as alias",
			fmt.Sprintf("Could not create send-as alias %s for user %s: %s", plan.SendAsEmail.ValueString(), userId, apiErrorDetail(err)),
		)
		return
	}

	mapSendAsToState(created, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *gmailSendAsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gmailSendAsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := state.UserId.ValueString()
	sendAsEmail := state.SendAsEmail.ValueString()

	apiSendAs, err := r.client.GetSendAs(ctx, userId, sendAsEmail, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Gmail send-as alias",
			fmt.Sprintf("Could not read send-as alias %s for user %s: %s", sendAsEmail, userId, apiErrorDetail(err)),
		)
		return
	}

	if apiSendAs == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapSendAsToState(apiSendAs, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *gmailSendAsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gmailSendAsResourceModel
	var state gmailSendAsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiSendAs := buildSendAsFromPlan(&plan)

	userId := state.UserId.ValueString()
	sendAsEmail := state.SendAsEmail.ValueString()

	updated, err := r.client.UpdateSendAs(ctx, userId, sendAsEmail, apiSendAs, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating Gmail send-as alias",
			fmt.Sprintf("Could not update send-as alias %s for user %s: %s", sendAsEmail, userId, apiErrorDetail(err)),
		)
		return
	}

	mapSendAsToState(updated, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *gmailSendAsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gmailSendAsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := state.UserId.ValueString()
	sendAsEmail := state.SendAsEmail.ValueString()

	err := r.client.DeleteSendAs(ctx, userId, sendAsEmail, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting Gmail send-as alias",
			fmt.Sprintf("Could not delete send-as alias %s for user %s: %s", sendAsEmail, userId, apiErrorDetail(err)),
		)
		return
	}
}

func (r *gmailSendAsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: user_id/send_as_email, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("send_as_email"), parts[1])...)
}

func buildSendAsFromPlan(plan *gmailSendAsResourceModel) *send_as.SendAs {
	s := &send_as.SendAs{
		SendAsEmail: plan.SendAsEmail.ValueString(),
	}

	if !plan.DisplayName.IsNull() && !plan.DisplayName.IsUnknown() {
		s.DisplayName = plan.DisplayName.ValueString()
	}
	if !plan.ReplyToAddress.IsNull() && !plan.ReplyToAddress.IsUnknown() {
		s.ReplyToAddress = plan.ReplyToAddress.ValueString()
	}
	if !plan.Signature.IsNull() && !plan.Signature.IsUnknown() {
		s.Signature = plan.Signature.ValueString()
	}
	if !plan.TreatAsAlias.IsNull() && !plan.TreatAsAlias.IsUnknown() {
		s.TreatAsAlias = plan.TreatAsAlias.ValueBool()
	}

	return s
}

func mapSendAsToState(s *send_as.SendAs, state *gmailSendAsResourceModel) {
	if s == nil {
		return
	}

	state.SendAsEmail = types.StringValue(s.SendAsEmail)

	// These are optional (not computed); map the API's empty strings back to null
	// so the result stays consistent with a config that leaves them unset.
	if s.DisplayName != "" {
		state.DisplayName = types.StringValue(s.DisplayName)
	} else {
		state.DisplayName = types.StringNull()
	}
	if s.ReplyToAddress != "" {
		state.ReplyToAddress = types.StringValue(s.ReplyToAddress)
	} else {
		state.ReplyToAddress = types.StringNull()
	}
	if s.Signature != "" {
		state.Signature = types.StringValue(s.Signature)
	} else {
		state.Signature = types.StringNull()
	}

	state.TreatAsAlias = types.BoolValue(s.TreatAsAlias)
	state.IsPrimary = types.BoolValue(s.IsPrimary)
	state.IsDefault = types.BoolValue(s.IsDefault)
	state.VerificationStatus = types.StringValue(s.VerificationStatus)
}
