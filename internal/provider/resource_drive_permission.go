package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/drive"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/drive/create_permission_config"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/drive/types/permission"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/drive/update_permission_config"
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
	_ resource.Resource                = &drivePermissionResource{}
	_ resource.ResourceWithImportState = &drivePermissionResource{}
)

const displayNameAttributeName = "display_name"

type drivePermissionResourceModel struct {
	Id                    types.String `tfsdk:"id"`
	FileId                types.String `tfsdk:"file_id"`
	Type                  types.String `tfsdk:"type"`
	EmailAddress          types.String `tfsdk:"email_address"`
	Domain                types.String `tfsdk:"domain"`
	Role                  types.String `tfsdk:"role"`
	AllowFileDiscovery    types.Bool   `tfsdk:"allow_file_discovery"`
	SendNotificationEmail types.Bool   `tfsdk:"send_notification_email"`
	DisplayName           types.String `tfsdk:"display_name"`
}

type drivePermissionResource struct {
	client       *drive.Client
	providerData *gwsProviderData
}

func NewDrivePermissionResource() resource.Resource {
	return &drivePermissionResource{}
}

func (r *drivePermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_drive_permission"
}

func (r *drivePermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a permission (share) on a Google Drive file, such as a spreadsheet.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique ID of the permission.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"file_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the Drive file (for a Google Sheet, the spreadsheet ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The grantee type. user, group, domain, or anyone.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email_address": schema.StringAttribute{
				Optional:    true,
				Description: "The email address of the user or group the permission refers to. Required for type user and group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				Optional:    true,
				Description: "The domain the permission refers to. Required for type domain.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Required:    true,
				Description: "The role granted by the permission. owner, organizer, fileOrganizer, writer, commenter, or reader. Ownership transfer (role owner) is not supported.",
			},
			"allow_file_discovery": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the file can be discovered through search. Only applicable for type domain and anyone.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"send_notification_email": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to send a notification email when creating the permission. Defaults to false (the Drive API default is true for user and group grants). Only used at creation time.",
			},
			displayNameAttributeName: schema.StringAttribute{
				Computed:    true,
				Description: "The displayable name of the grantee.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *drivePermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := getProviderData(req.ProviderData, &resp.Diagnostics)
	if data == nil {
		return
	}
	r.client = data.driveClient
	r.providerData = data
}

func (r *drivePermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan drivePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPermission := &permission.Permission{
		Type:               plan.Type.ValueString(),
		EmailAddress:       plan.EmailAddress.ValueString(),
		Domain:             plan.Domain.ValueString(),
		Role:               plan.Role.ValueString(),
		AllowFileDiscovery: plan.AllowFileDiscovery.ValueBool(),
	}

	fileId := plan.FileId.ValueString()
	createdPermission, err := r.client.CreatePermission(
		ctx,
		fileId,
		apiPermission,
		create_permission_config.WithSendNotificationEmail(plan.SendNotificationEmail.ValueBool()),
		create_permission_config.WithFetchOptions(fetchOptions(r.providerData)...),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Drive permission",
			fmt.Sprintf("Could not create permission on file %s: %s", fileId, apiErrorDetail(err)),
		)
		return
	}

	mapDrivePermissionToState(createdPermission, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *drivePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state drivePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fileId := state.FileId.ValueString()
	permissionId := state.Id.ValueString()

	apiPermission, err := r.client.GetPermission(ctx, fileId, permissionId, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Drive permission",
			fmt.Sprintf("Could not read permission %s on file %s: %s", permissionId, fileId, apiErrorDetail(err)),
		)
		return
	}

	if apiPermission == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapDrivePermissionToState(apiPermission, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *drivePermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan drivePermissionResourceModel
	var state drivePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fileId := state.FileId.ValueString()
	permissionId := state.Id.ValueString()

	updatedPermission, err := r.client.UpdatePermission(
		ctx,
		fileId,
		permissionId,
		&permission.Permission{Role: plan.Role.ValueString()},
		update_permission_config.WithFetchOptions(fetchOptions(r.providerData)...),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating Drive permission",
			fmt.Sprintf("Could not update permission %s on file %s: %s", permissionId, fileId, apiErrorDetail(err)),
		)
		return
	}

	mapDrivePermissionToState(updatedPermission, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *drivePermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state drivePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fileId := state.FileId.ValueString()
	permissionId := state.Id.ValueString()

	err := r.client.DeletePermission(ctx, fileId, permissionId, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting Drive permission",
			fmt.Sprintf("Could not delete permission %s on file %s: %s", permissionId, fileId, apiErrorDetail(err)),
		)
		return
	}
}

func (r *drivePermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: file_id/permission_id (grantee email also accepted), got: %s", req.ID),
		)
		return
	}
	fileId := parts[0]
	permissionKey := parts[1]

	// Permission IDs are opaque, so also accept the grantee's email address and
	// resolve it via the permission list.
	if strings.Contains(permissionKey, "@") {
		permissions, err := r.client.ListPermissions(ctx, fileId, fetchOptions(r.providerData)...)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error listing Drive permissions for import",
				fmt.Sprintf("Could not list permissions on file %s: %s", fileId, apiErrorDetail(err)),
			)
			return
		}

		var match *permission.Permission
		for _, apiPermission := range permissions {
			if strings.EqualFold(apiPermission.EmailAddress, permissionKey) {
				match = apiPermission
				break
			}
		}
		if match == nil {
			resp.Diagnostics.AddError(
				"Drive permission not found",
				fmt.Sprintf("No permission for %q found on file %s.", permissionKey, fileId),
			)
			return
		}
		permissionKey = match.Id
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("file_id"), fileId)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), permissionKey)...)
	// send_notification_email only matters at creation time and is not part of
	// the remote state; seed the schema default to avoid a spurious first diff.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("send_notification_email"), false)...)
}

// mapDrivePermissionToState copies the API permission into the state model.
// file_id and send_notification_email are not part of the API resource and are
// left untouched.
func mapDrivePermissionToState(apiPermission *permission.Permission, state *drivePermissionResourceModel) {
	if apiPermission == nil {
		return
	}

	state.Id = types.StringValue(apiPermission.Id)
	state.Type = types.StringValue(apiPermission.Type)
	state.Role = types.StringValue(apiPermission.Role)
	state.AllowFileDiscovery = types.BoolValue(apiPermission.AllowFileDiscovery)
	state.DisplayName = types.StringValue(apiPermission.DisplayName)

	if apiPermission.EmailAddress != "" {
		state.EmailAddress = types.StringValue(apiPermission.EmailAddress)
	} else {
		state.EmailAddress = types.StringNull()
	}
	if apiPermission.Domain != "" {
		state.Domain = types.StringValue(apiPermission.Domain)
	} else {
		state.Domain = types.StringNull()
	}
}
