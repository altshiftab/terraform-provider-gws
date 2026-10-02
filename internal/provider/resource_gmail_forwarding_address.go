package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/gmail"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/gmail/types/forwarding_address"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &gmailForwardingAddressResource{}
	_ resource.ResourceWithImportState = &gmailForwardingAddressResource{}
)

type gmailForwardingAddressResourceModel struct {
	UserId             types.String `tfsdk:"user_id"`
	ForwardingEmail    types.String `tfsdk:"forwarding_email"`
	VerificationStatus types.String `tfsdk:"verification_status"`
}

type gmailForwardingAddressResource struct {
	client       *gmail.Client
	providerData *gwsProviderData
}

func NewGmailForwardingAddressResource() resource.Resource {
	return &gmailForwardingAddressResource{}
}

func (r *gmailForwardingAddressResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gmail_forwarding_address"
}

func (r *gmailForwardingAddressResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an address a Gmail user may forward mail to, as named by a filter's forward action. " +
			"An address outside the user's domain is created pending: Gmail mails it a confirmation, and forwarding " +
			"to it is refused until someone there confirms. The API cannot do that step. An address that already " +
			"exists is brought under management with terraform import (user_id/forwarding_email), not by creating it.",
		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "The user's email address or the special value \"me\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"forwarding_email": schema.StringAttribute{
				Required:    true,
				Description: "The address mail may be forwarded to. Gmail has no update for it, so a change replaces it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"verification_status": schema.StringAttribute{
				Computed:    true,
				Description: "\"pending\" until the confirmation Gmail sent to the address is acted on, then \"accepted\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *gmailForwardingAddressResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := getProviderData(req.ProviderData, &resp.Diagnostics)
	if data == nil {
		return
	}
	r.client = data.gmailClient
	r.providerData = data
}

func (r *gmailForwardingAddressResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gmailForwardingAddressResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := plan.UserId.ValueString()
	forwardingEmail := plan.ForwardingEmail.ValueString()

	created, err := r.client.CreateForwardingAddress(
		ctx,
		userId,
		&forwarding_address.ForwardingAddress{ForwardingEmail: forwardingEmail},
		fetchOptions(r.providerData)...,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Gmail forwarding address",
			fmt.Sprintf("Could not create forwarding address %s for user %s: %s", forwardingEmail, userId, apiErrorDetail(err)),
		)
		return
	}

	mapForwardingAddressToState(created, &plan)
	warnIfPending(&plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *gmailForwardingAddressResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gmailForwardingAddressResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := state.UserId.ValueString()
	forwardingEmail := state.ForwardingEmail.ValueString()

	apiAddress, err := r.client.GetForwardingAddress(ctx, userId, forwardingEmail, fetchOptions(r.providerData)...)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error reading Gmail forwarding address",
			fmt.Sprintf("Could not read forwarding address %s for user %s: %s", forwardingEmail, userId, apiErrorDetail(err)),
		)
		return
	}

	if apiAddress == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapForwardingAddressToState(apiAddress, &state)
	warnIfPending(&state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update is never reached: both configurable attributes require replacement.
func (r *gmailForwardingAddressResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gmailForwardingAddressResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *gmailForwardingAddressResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gmailForwardingAddressResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := state.UserId.ValueString()
	forwardingEmail := state.ForwardingEmail.ValueString()

	err := r.client.DeleteForwardingAddress(ctx, userId, forwardingEmail, fetchOptions(r.providerData)...)
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError(
			"Error deleting Gmail forwarding address",
			fmt.Sprintf("Could not delete forwarding address %s for user %s: %s", forwardingEmail, userId, apiErrorDetail(err)),
		)
	}
}

func (r *gmailForwardingAddressResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: user_id/forwarding_email, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("forwarding_email"), parts[1])...)
}

func mapForwardingAddressToState(f *forwarding_address.ForwardingAddress, state *gmailForwardingAddressResourceModel) {
	if f == nil {
		return
	}

	// Keep the configured spelling when Gmail answers with another case of it, which would otherwise read
	// as an inconsistent result after apply and a replacement on every plan.
	if !strings.EqualFold(f.ForwardingEmail, state.ForwardingEmail.ValueString()) {
		state.ForwardingEmail = types.StringValue(f.ForwardingEmail)
	}
	state.VerificationStatus = types.StringValue(string(f.VerificationStatus))
}

// warnIfPending says, on every plan and apply, that a filter cannot forward to the address yet.
func warnIfPending(state *gmailForwardingAddressResourceModel, diagnostics *diag.Diagnostics) {
	if state.VerificationStatus.ValueString() != string(forwarding_address.VerificationStatusPending) {
		return
	}

	diagnostics.AddWarning(
		"Gmail forwarding address awaits confirmation",
		fmt.Sprintf(
			"Gmail has mailed a confirmation to %s. Until someone there opens its link, or enters its code "+
				"(also in the subject line) in Gmail's forwarding settings, a filter that forwards to it is refused.",
			state.ForwardingEmail.ValueString(),
		),
	)
}
