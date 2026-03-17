package provider

import (
	"context"
	"fmt"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory/types/group"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &groupResource{}
	_ resource.ResourceWithImportState = &groupResource{}
	_ resource.ResourceWithModifyPlan  = &groupResource{}
)

type groupResourceModel struct {
	Id                 types.String `tfsdk:"id"`
	Email              types.String `tfsdk:"email"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	Etag               types.String `tfsdk:"etag"`
	DirectMembersCount types.String `tfsdk:"direct_members_count"`
	AdminCreated       types.Bool   `tfsdk:"admin_created"`
	Aliases            types.List   `tfsdk:"aliases"`
	NonEditableAliases types.List   `tfsdk:"non_editable_aliases"`
}

type groupResource struct {
	client       *directory.Client
	providerData *gwsProviderData
}

func NewGroupResource() resource.Resource {
	return &groupResource{}
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Google Workspace group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique ID of the group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Required:    true,
				Description: "The group's email address.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The group's display name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The group's description.",
			},
			"etag": schema.StringAttribute{
				Computed:    true,
				Description: "ETag of the resource.",
			},
			"direct_members_count": schema.StringAttribute{
				Computed:    true,
				Description: "Number of direct members in the group.",
			},
			"admin_created": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the group was created by an administrator.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"aliases": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Aliases for the group.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"non_editable_aliases": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Non-editable aliases for the group.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := getProviderData(req.ProviderData, &resp.Diagnostics)
	if data == nil {
		return
	}
	r.client = data.directoryClient
	r.providerData = data
}

func (r *groupResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip during create or destroy.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state, plan groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If any user-configurable field changed, mark volatile computed fields
	// as unknown so Terraform accepts the new values after apply.
	if !state.Email.Equal(plan.Email) || !state.Name.Equal(plan.Name) || !state.Description.Equal(plan.Description) {
		plan.Etag = types.StringUnknown()
		plan.DirectMembersCount = types.StringUnknown()
		resp.Diagnostics.Append(resp.Plan.Set(ctx, plan)...)
	}
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiGroup := &group.Group{
		Email:       plan.Email.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}

	createdGroup, err := r.client.CreateGroup(ctx, apiGroup, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating group",
			fmt.Sprintf("Could not create group %s: %s", plan.Email.ValueString(), err),
		)
		return
	}

	mapGroupToState(ctx, createdGroup, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupKey := state.Email.ValueString()
	if groupKey == "" {
		groupKey = state.Id.ValueString()
	}

	apiGroup, err := r.client.GetGroup(ctx, groupKey, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading group",
			fmt.Sprintf("Could not read group %s: %s", groupKey, err),
		)
		return
	}

	if apiGroup == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapGroupToState(ctx, apiGroup, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupResourceModel
	var state groupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupKey := state.Email.ValueString()
	if groupKey == "" {
		groupKey = state.Id.ValueString()
	}

	apiGroup := &group.Group{
		Email:       plan.Email.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}

	_, err := r.client.UpdateGroup(ctx, groupKey, apiGroup, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating group",
			fmt.Sprintf("Could not update group %s: %s", groupKey, err),
		)
		return
	}

	// Read the full group after update since the Update API response
	// may not include all fields (e.g. direct_members_count).
	readKey := plan.Email.ValueString()
	if readKey == "" {
		readKey = state.Id.ValueString()
	}

	fullGroup, err := r.client.GetGroup(ctx, readKey, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading group after update",
			fmt.Sprintf("Could not read group %s: %s", readKey, err),
		)
		return
	}

	mapGroupToState(ctx, fullGroup, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupKey := state.Id.ValueString()
	if groupKey == "" {
		groupKey = state.Email.ValueString()
	}

	err := r.client.DeleteGroup(ctx, groupKey, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting group",
			fmt.Sprintf("Could not delete group %s: %s", groupKey, err),
		)
		return
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("email"), req, resp)
}

func mapGroupToState(ctx context.Context, g *group.Group, state *groupResourceModel) {
	if g == nil {
		return
	}

	state.Id = types.StringValue(g.Id)
	state.Email = types.StringValue(g.Email)
	state.Name = types.StringValue(g.Name)

	if g.Description != "" {
		state.Description = types.StringValue(g.Description)
	} else {
		state.Description = types.StringNull()
	}
	state.Etag = types.StringValue(g.Etag)
	state.DirectMembersCount = types.StringValue(g.DirectMembersCount)
	state.AdminCreated = types.BoolValue(g.AdminCreated)

	if len(g.Aliases) > 0 {
		aliases, _ := types.ListValueFrom(ctx, types.StringType, g.Aliases)
		state.Aliases = aliases
	} else {
		state.Aliases = types.ListNull(types.StringType)
	}

	if len(g.NonEditableAliases) > 0 {
		nonEditableAliases, _ := types.ListValueFrom(ctx, types.StringType, g.NonEditableAliases)
		state.NonEditableAliases = nonEditableAliases
	} else {
		state.NonEditableAliases = types.ListNull(types.StringType)
	}
}
