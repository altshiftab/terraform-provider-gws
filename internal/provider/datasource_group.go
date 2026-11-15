package provider

import (
	"context"
	"fmt"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/directory"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &groupDataSource{}

type groupDataSourceModel struct {
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

type groupDataSource struct {
	client       *directory.Client
	providerData *gwsProviderData
}

func NewGroupDataSource() datasource.DataSource {
	return &groupDataSource{}
}

func (d *groupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *groupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attributes := groupDataSourceAttributes(true)
	attributes["etag"] = schema.StringAttribute{
		Computed:    true,
		Description: "ETag of the resource.",
	}
	attributes["aliases"] = schema.ListAttribute{
		Computed:    true,
		ElementType: types.StringType,
		Description: "Aliases for the group.",
	}
	attributes["non_editable_aliases"] = schema.ListAttribute{
		Computed:    true,
		ElementType: types.StringType,
		Description: "Non-editable aliases for the group.",
	}

	resp.Schema = schema.Schema{
		Description: "Fetches a Google Workspace group.",
		Attributes:  attributes,
	}
}

// groupDataSourceAttributes describes a group as both gws_group and the entries of gws_groups present one; gws_group
// adds what only fetching the one group returns. gws_group looks the group up by email, which is then required.
func groupDataSourceAttributes(emailRequired bool) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The unique ID of the group.",
		},
		"email": schema.StringAttribute{
			Required:    emailRequired,
			Computed:    !emailRequired,
			Description: "The group's email address.",
		},
		"name": schema.StringAttribute{
			Computed:    true,
			Description: "The group's display name.",
		},
		"description": schema.StringAttribute{
			Computed:    true,
			Description: "The group's description.",
		},
		"direct_members_count": schema.StringAttribute{
			Computed:    true,
			Description: "Number of direct members in the group.",
		},
		"admin_created": schema.BoolAttribute{
			Computed:    true,
			Description: "Whether the group was created by an administrator.",
		},
	}
}

func (d *groupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *groupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config groupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupKey := config.Email.ValueString()

	apiGroup, err := d.client.GetGroup(ctx, groupKey, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading group",
			fmt.Sprintf("Could not read group %s: %s", groupKey, apiErrorDetail(err)),
		)
		return
	}

	if apiGroup == nil {
		resp.Diagnostics.AddError(
			"Group not found",
			fmt.Sprintf("Group %s not found.", groupKey),
		)
		return
	}

	config.Id = types.StringValue(apiGroup.Id)
	config.Email = types.StringValue(apiGroup.Email)
	config.Name = types.StringValue(apiGroup.Name)
	config.Etag = types.StringValue(apiGroup.Etag)
	config.DirectMembersCount = types.StringValue(apiGroup.DirectMembersCount)
	config.AdminCreated = types.BoolValue(apiGroup.AdminCreated)

	if apiGroup.Description != "" {
		config.Description = types.StringValue(apiGroup.Description)
	} else {
		config.Description = types.StringNull()
	}

	if len(apiGroup.Aliases) > 0 {
		aliases, _ := types.ListValueFrom(ctx, types.StringType, apiGroup.Aliases)
		config.Aliases = aliases
	} else {
		config.Aliases = types.ListNull(types.StringType)
	}

	if len(apiGroup.NonEditableAliases) > 0 {
		nonEditableAliases, _ := types.ListValueFrom(ctx, types.StringType, apiGroup.NonEditableAliases)
		config.NonEditableAliases = nonEditableAliases
	} else {
		config.NonEditableAliases = types.ListNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
