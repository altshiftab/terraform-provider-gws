package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory/types/member"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &groupMemberResource{}
	_ resource.ResourceWithImportState = &groupMemberResource{}
)

type groupMemberResourceModel struct {
	Id               types.String `tfsdk:"id"`
	GroupKey          types.String `tfsdk:"group_key"`
	Email            types.String `tfsdk:"email"`
	Role             types.String `tfsdk:"role"`
	Type             types.String `tfsdk:"type"`
	DeliverySettings types.String `tfsdk:"delivery_settings"`
	Etag             types.String `tfsdk:"etag"`
	Status           types.String `tfsdk:"status"`
}

type groupMemberResource struct {
	client       *directory.Client
	providerData *gwsProviderData
}

func NewGroupMemberResource() resource.Resource {
	return &groupMemberResource{}
}

func (r *groupMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_member"
}

func (r *groupMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a member of a Google Workspace group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique ID of the group member.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_key": schema.StringAttribute{
				Required:    true,
				Description: "The group's email address or unique ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				Required:    true,
				Description: "The member's email address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("MEMBER"),
				Description: "The member's role in the group. OWNER, MANAGER, or MEMBER.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "The type of group member (USER, GROUP, etc.).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"delivery_settings": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Delivery settings for the member. ALL_MAIL, DAILY, DIGEST, DISABLED, or NONE.",
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
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "The status of the member.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *groupMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := getProviderData(req.ProviderData, &resp.Diagnostics)
	if data == nil {
		return
	}
	r.client = data.directoryClient
	r.providerData = data
}

func (r *groupMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiMember := &member.Member{
		Email:            plan.Email.ValueString(),
		Role:             plan.Role.ValueString(),
		DeliverySettings: plan.DeliverySettings.ValueString(),
	}

	groupKey := plan.GroupKey.ValueString()
	createdMember, err := r.client.CreateMember(ctx, groupKey, apiMember, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating group member",
			fmt.Sprintf("Could not add %s to group %s: %s", plan.Email.ValueString(), groupKey, err),
		)
		return
	}

	mapMemberToState(createdMember, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupKey := state.GroupKey.ValueString()
	memberKey := state.Email.ValueString()
	if memberKey == "" {
		memberKey = state.Id.ValueString()
	}

	apiMember, err := r.client.GetMember(ctx, groupKey, memberKey, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading group member",
			fmt.Sprintf("Could not read member %s in group %s: %s", memberKey, groupKey, err),
		)
		return
	}

	if apiMember == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapMemberToState(apiMember, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *groupMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupMemberResourceModel
	var state groupMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupKey := state.GroupKey.ValueString()
	memberKey := state.Email.ValueString()
	if memberKey == "" {
		memberKey = state.Id.ValueString()
	}

	apiMember := &member.Member{
		Email:            plan.Email.ValueString(),
		Role:             plan.Role.ValueString(),
		DeliverySettings: plan.DeliverySettings.ValueString(),
	}

	updatedMember, err := r.client.UpdateMember(ctx, groupKey, memberKey, apiMember, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating group member",
			fmt.Sprintf("Could not update member %s in group %s: %s", memberKey, groupKey, err),
		)
		return
	}

	mapMemberToState(updatedMember, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupKey := state.GroupKey.ValueString()
	memberKey := state.Id.ValueString()
	if memberKey == "" {
		memberKey = state.Email.ValueString()
	}

	err := r.client.DeleteMember(ctx, groupKey, memberKey, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting group member",
			fmt.Sprintf("Could not remove member %s from group %s: %s", memberKey, groupKey, err),
		)
		return
	}
}

func (r *groupMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: group_email/member_email, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_key"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("email"), parts[1])...)
}

func mapMemberToState(m *member.Member, state *groupMemberResourceModel) {
	if m == nil {
		return
	}

	state.Id = types.StringValue(m.Id)
	state.Email = types.StringValue(m.Email)
	state.Role = types.StringValue(m.Role)
	state.Type = types.StringValue(m.Type)
	state.Status = types.StringValue(m.Status)
	state.Etag = types.StringValue(m.Etag)
	state.DeliverySettings = types.StringValue(m.DeliverySettings)
}
