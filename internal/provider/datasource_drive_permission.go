package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/drive"
	"github.com/altshiftab/utils_go/pkg/cloud/gws/drive/types/permission"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &drivePermissionDataSource{}

type drivePermissionDataSourceModel struct {
	Id                 types.String `tfsdk:"id"`
	FileId             types.String `tfsdk:"file_id"`
	PermissionId       types.String `tfsdk:"permission_id"`
	EmailAddress       types.String `tfsdk:"email_address"`
	Type               types.String `tfsdk:"type"`
	Domain             types.String `tfsdk:"domain"`
	Role               types.String `tfsdk:"role"`
	AllowFileDiscovery types.Bool   `tfsdk:"allow_file_discovery"`
	DisplayName        types.String `tfsdk:"display_name"`
	Deleted            types.Bool   `tfsdk:"deleted"`
}

type drivePermissionDataSource struct {
	client       *drive.Client
	providerData *gwsProviderData
}

func NewDrivePermissionDataSource() datasource.DataSource {
	return &drivePermissionDataSource{}
}

func (d *drivePermissionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_drive_permission"
}

func (d *drivePermissionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a permission on a Google Drive file, looked up by permission ID or grantee email address.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique ID of the permission.",
			},
			"file_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the Drive file (for a Google Sheet, the spreadsheet ID).",
			},
			"permission_id": schema.StringAttribute{
				Optional:    true,
				Description: "The ID of the permission. Exactly one of permission_id and email_address must be set.",
			},
			"email_address": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The email address of the user or group the permission refers to. Exactly one of permission_id and email_address must be set.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "The grantee type. user, group, domain, or anyone.",
			},
			"domain": schema.StringAttribute{
				Computed:    true,
				Description: "The domain the permission refers to.",
			},
			"role": schema.StringAttribute{
				Computed:    true,
				Description: "The role granted by the permission.",
			},
			"allow_file_discovery": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the file can be discovered through search.",
			},
			displayNameAttributeName: schema.StringAttribute{
				Computed:    true,
				Description: "The displayable name of the grantee.",
			},
			"deleted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the grantee account has been deleted.",
			},
		},
	}
}

func (d *drivePermissionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = data.driveClient
	d.providerData = data
}

func (d *drivePermissionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config drivePermissionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fileId := config.FileId.ValueString()
	permissionId := config.PermissionId.ValueString()
	emailAddress := config.EmailAddress.ValueString()

	if (permissionId == "") == (emailAddress == "") {
		resp.Diagnostics.AddError(
			"Invalid Drive permission lookup",
			"Exactly one of permission_id and email_address must be set.",
		)
		return
	}

	var apiPermission *permission.Permission
	if permissionId != "" {
		var err error
		apiPermission, err = d.client.GetPermission(ctx, fileId, permissionId, fetchOptions(d.providerData)...)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading Drive permission",
				fmt.Sprintf("Could not read permission %s on file %s: %s", permissionId, fileId, apiErrorDetail(err)),
			)
			return
		}
	} else {
		permissions, err := d.client.ListPermissions(ctx, fileId, fetchOptions(d.providerData)...)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error listing Drive permissions",
				fmt.Sprintf("Could not list permissions on file %s: %s", fileId, apiErrorDetail(err)),
			)
			return
		}

		for _, candidate := range permissions {
			if strings.EqualFold(candidate.EmailAddress, emailAddress) {
				apiPermission = candidate
				break
			}
		}
	}

	if apiPermission == nil {
		resp.Diagnostics.AddError(
			"Drive permission not found",
			fmt.Sprintf("No matching permission found on file %s.", fileId),
		)
		return
	}

	config.Id = types.StringValue(apiPermission.Id)
	config.Type = types.StringValue(apiPermission.Type)
	config.Role = types.StringValue(apiPermission.Role)
	config.AllowFileDiscovery = types.BoolValue(apiPermission.AllowFileDiscovery)
	config.DisplayName = types.StringValue(apiPermission.DisplayName)
	config.Deleted = types.BoolValue(apiPermission.Deleted)
	if apiPermission.EmailAddress != "" {
		config.EmailAddress = types.StringValue(apiPermission.EmailAddress)
	} else {
		config.EmailAddress = types.StringNull()
	}
	if apiPermission.Domain != "" {
		config.Domain = types.StringValue(apiPermission.Domain)
	} else {
		config.Domain = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
