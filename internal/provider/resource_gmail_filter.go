package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/gmail"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/gmail/types/filter"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &gmailFilterResource{}
	_ resource.ResourceWithImportState = &gmailFilterResource{}
)

type gmailFilterCriteriaModel struct {
	From           types.String `tfsdk:"from"`
	To             types.String `tfsdk:"to"`
	Subject        types.String `tfsdk:"subject"`
	Query          types.String `tfsdk:"query"`
	NegatedQuery   types.String `tfsdk:"negated_query"`
	HasAttachment  types.Bool   `tfsdk:"has_attachment"`
	ExcludeChats   types.Bool   `tfsdk:"exclude_chats"`
	Size           types.Int64  `tfsdk:"size"`
	SizeComparison types.String `tfsdk:"size_comparison"`
}

type gmailFilterActionModel struct {
	AddLabelIds    types.List   `tfsdk:"add_label_ids"`
	RemoveLabelIds types.List   `tfsdk:"remove_label_ids"`
	Forward        types.String `tfsdk:"forward"`
}

type gmailFilterResourceModel struct {
	UserId   types.String              `tfsdk:"user_id"`
	FilterId types.String              `tfsdk:"filter_id"`
	Criteria *gmailFilterCriteriaModel `tfsdk:"criteria"`
	Action   *gmailFilterActionModel   `tfsdk:"action"`
}

type gmailFilterResource struct {
	client       *gmail.Client
	providerData *gwsProviderData
}

func NewGmailFilterResource() resource.Resource {
	return &gmailFilterResource{}
}

func (r *gmailFilterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gmail_filter"
}

func (r *gmailFilterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Gmail filter for a Google Workspace user. Filters are immutable in the Gmail API; any change forces replacement.",
		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "The user's email address or the special value \"me\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"filter_id": schema.StringAttribute{
				Computed:    true,
				Description: "The server-assigned identifier of the filter.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"criteria": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Matching criteria for the filter.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"from": schema.StringAttribute{
						Optional:    true,
						Description: "The sender's display name or email address.",
					},
					"to": schema.StringAttribute{
						Optional:    true,
						Description: "The recipient's display name or email address.",
					},
					"subject": schema.StringAttribute{
						Optional:    true,
						Description: "Case-insensitive phrase found in the message's subject.",
					},
					"query": schema.StringAttribute{
						Optional:    true,
						Description: "Gmail search query (e.g. \"from:someuser@example.com is:unread\").",
					},
					"negated_query": schema.StringAttribute{
						Optional:    true,
						Description: "Gmail search query that must NOT match.",
					},
					"has_attachment": schema.BoolAttribute{
						Optional:    true,
						Description: "Whether the message has any attachment.",
					},
					"exclude_chats": schema.BoolAttribute{
						Optional:    true,
						Description: "Whether chat messages should be excluded.",
					},
					"size": schema.Int64Attribute{
						Optional:    true,
						Description: "Size of the entire RFC822 message in bytes, compared against size_comparison.",
					},
					"size_comparison": schema.StringAttribute{
						Optional:    true,
						Description: "How the message size is compared (\"smaller\" or \"larger\").",
					},
				},
			},
			"action": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Action to apply to matching messages.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"add_label_ids": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Labels to add to matching messages.",
					},
					"remove_label_ids": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Labels to remove from matching messages.",
					},
					"forward": schema.StringAttribute{
						Optional:    true,
						Description: "Email address to forward matching messages to. Must be a verified forwarding address.",
					},
				},
			},
		},
	}
}

func (r *gmailFilterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := getProviderData(req.ProviderData, &resp.Diagnostics)
	if data == nil {
		return
	}
	r.client = data.gmailClient
	r.providerData = data
}

func (r *gmailFilterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gmailFilterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiFilter, diags := buildFilterFromPlan(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := plan.UserId.ValueString()
	created, err := r.client.CreateFilter(ctx, userId, apiFilter, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Gmail filter",
			fmt.Sprintf("Could not create filter for user %s: %s", userId, apiErrorDetail(err)),
		)
		return
	}

	resp.Diagnostics.Append(mapFilterToResourceState(ctx, created, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *gmailFilterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gmailFilterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := state.UserId.ValueString()
	filterId := state.FilterId.ValueString()

	apiFilter, err := r.client.GetFilter(ctx, userId, filterId, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Gmail filter",
			fmt.Sprintf("Could not read filter %s for user %s: %s", filterId, userId, apiErrorDetail(err)),
		)
		return
	}

	if apiFilter == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(mapFilterToResourceState(ctx, apiFilter, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *gmailFilterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Gmail filters are immutable; all mutable attributes force replacement, so Update should never be invoked.
	var plan gmailFilterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *gmailFilterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gmailFilterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := state.UserId.ValueString()
	filterId := state.FilterId.ValueString()

	err := r.client.DeleteFilter(ctx, userId, filterId, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting Gmail filter",
			fmt.Sprintf("Could not delete filter %s for user %s: %s", filterId, userId, apiErrorDetail(err)),
		)
		return
	}
}

func (r *gmailFilterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: user_id/filter_id, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("filter_id"), parts[1])...)
}

func buildFilterFromPlan(ctx context.Context, plan *gmailFilterResourceModel) (*filter.Filter, diag.Diagnostics) {
	var diags diag.Diagnostics

	f := &filter.Filter{}

	if plan.Criteria != nil {
		c := &filter.Criteria{}
		if !plan.Criteria.From.IsNull() && !plan.Criteria.From.IsUnknown() {
			c.From = plan.Criteria.From.ValueString()
		}
		if !plan.Criteria.To.IsNull() && !plan.Criteria.To.IsUnknown() {
			c.To = plan.Criteria.To.ValueString()
		}
		if !plan.Criteria.Subject.IsNull() && !plan.Criteria.Subject.IsUnknown() {
			c.Subject = plan.Criteria.Subject.ValueString()
		}
		if !plan.Criteria.Query.IsNull() && !plan.Criteria.Query.IsUnknown() {
			c.Query = plan.Criteria.Query.ValueString()
		}
		if !plan.Criteria.NegatedQuery.IsNull() && !plan.Criteria.NegatedQuery.IsUnknown() {
			c.NegatedQuery = plan.Criteria.NegatedQuery.ValueString()
		}
		if !plan.Criteria.HasAttachment.IsNull() && !plan.Criteria.HasAttachment.IsUnknown() {
			c.HasAttachment = plan.Criteria.HasAttachment.ValueBool()
		}
		if !plan.Criteria.ExcludeChats.IsNull() && !plan.Criteria.ExcludeChats.IsUnknown() {
			c.ExcludeChats = plan.Criteria.ExcludeChats.ValueBool()
		}
		if !plan.Criteria.Size.IsNull() && !plan.Criteria.Size.IsUnknown() {
			c.Size = int(plan.Criteria.Size.ValueInt64())
		}
		if !plan.Criteria.SizeComparison.IsNull() && !plan.Criteria.SizeComparison.IsUnknown() {
			c.SizeComparison = filter.SizeComparison(plan.Criteria.SizeComparison.ValueString())
		}
		f.Criteria = c
	}

	if plan.Action != nil {
		a := &filter.Action{}
		if !plan.Action.AddLabelIds.IsNull() && !plan.Action.AddLabelIds.IsUnknown() {
			var ids []string
			d := plan.Action.AddLabelIds.ElementsAs(ctx, &ids, false)
			diags = append(diags, d...)
			a.AddLabelIds = ids
		}
		if !plan.Action.RemoveLabelIds.IsNull() && !plan.Action.RemoveLabelIds.IsUnknown() {
			var ids []string
			d := plan.Action.RemoveLabelIds.ElementsAs(ctx, &ids, false)
			diags = append(diags, d...)
			a.RemoveLabelIds = ids
		}
		if !plan.Action.Forward.IsNull() && !plan.Action.Forward.IsUnknown() {
			a.Forward = plan.Action.Forward.ValueString()
		}
		f.Action = a
	}

	return f, diags
}

func mapFilterToResourceState(ctx context.Context, f *filter.Filter, state *gmailFilterResourceModel) diag.Diagnostics {
	if f == nil {
		return nil
	}

	state.FilterId = types.StringValue(f.Id)

	var diags diag.Diagnostics
	state.Criteria = mapCriteriaToModel(f.Criteria)
	action, d := mapActionToModel(ctx, f.Action)
	diags = append(diags, d...)
	state.Action = action

	return diags
}

func mapCriteriaToModel(c *filter.Criteria) *gmailFilterCriteriaModel {
	if c == nil {
		return nil
	}

	m := &gmailFilterCriteriaModel{
		From:           stringValueOrNull(c.From),
		To:             stringValueOrNull(c.To),
		Subject:        stringValueOrNull(c.Subject),
		Query:          stringValueOrNull(c.Query),
		NegatedQuery:   stringValueOrNull(c.NegatedQuery),
		HasAttachment:  boolValueOrNull(c.HasAttachment),
		ExcludeChats:   boolValueOrNull(c.ExcludeChats),
		SizeComparison: stringValueOrNull(string(c.SizeComparison)),
	}
	if c.Size != 0 {
		m.Size = types.Int64Value(int64(c.Size))
	} else {
		m.Size = types.Int64Null()
	}
	return m
}

func mapActionToModel(ctx context.Context, a *filter.Action) (*gmailFilterActionModel, diag.Diagnostics) {
	if a == nil {
		return nil, nil
	}

	var diags diag.Diagnostics
	m := &gmailFilterActionModel{
		Forward: stringValueOrNull(a.Forward),
	}

	if len(a.AddLabelIds) > 0 {
		list, d := types.ListValueFrom(ctx, types.StringType, a.AddLabelIds)
		diags = append(diags, d...)
		m.AddLabelIds = list
	} else {
		m.AddLabelIds = types.ListNull(types.StringType)
	}

	if len(a.RemoveLabelIds) > 0 {
		list, d := types.ListValueFrom(ctx, types.StringType, a.RemoveLabelIds)
		diags = append(diags, d...)
		m.RemoveLabelIds = list
	} else {
		m.RemoveLabelIds = types.ListNull(types.StringType)
	}

	return m, diags
}

func stringValueOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func boolValueOrNull(b bool) types.Bool {
	if !b {
		return types.BoolNull()
	}
	return types.BoolValue(b)
}
