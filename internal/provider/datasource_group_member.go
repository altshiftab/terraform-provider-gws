package provider

import (
	"context"
	"fmt"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &groupMemberDataSource{}

type groupMemberDataSourceModel struct {
	Id               types.String `tfsdk:"id"`
	GroupKey         types.String `tfsdk:"group_key"`
	Email            types.String `tfsdk:"email"`
	Role             types.String `tfsdk:"role"`
	Type             types.String `tfsdk:"type"`
	DeliverySettings types.String `tfsdk:"delivery_settings"`
	Etag             types.String `tfsdk:"etag"`
	Status           types.String `tfsdk:"status"`
}

type groupMemberDataSource struct {
	client       *directory.Client
	providerData *gwsProviderData
}

func NewGroupMemberDataSource() datasource.DataSource {
	return &groupMemberDataSource{}
}

func (d *groupMemberDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_member"
}

func (d *groupMemberDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a member of a Google Workspace group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique ID of the group member.",
			},
			"group_key": schema.StringAttribute{
				Required:    true,
				Description: "The group's email address or unique ID.",
			},
			"email": schema.StringAttribute{
				Required:    true,
				Description: "The member's email address.",
			},
			"role": schema.StringAttribute{
				Computed:    true,
				Description: "The member's role in the group. OWNER, MANAGER, or MEMBER.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "The type of group member (USER, GROUP, etc.).",
			},
			"delivery_settings": schema.StringAttribute{
				Computed:    true,
				Description: "Delivery settings for the member. ALL_MAIL, DAILY, DIGEST, DISABLED, or NONE.",
			},
			"etag": schema.StringAttribute{
				Computed:    true,
				Description: "ETag of the resource.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "The status of the member.",
			},
		},
	}
}

func (d *groupMemberDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *groupMemberDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config groupMemberDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupKey := config.GroupKey.ValueString()
	memberKey := config.Email.ValueString()

	apiMember, err := d.client.GetMember(ctx, groupKey, memberKey, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading group member",
			fmt.Sprintf("Could not read member %s in group %s: %s", memberKey, groupKey, apiErrorDetail(err)),
		)
		return
	}

	if apiMember == nil {
		resp.Diagnostics.AddError(
			"Group member not found",
			fmt.Sprintf("Member %s not found in group %s.", memberKey, groupKey),
		)
		return
	}

	config.Id = types.StringValue(apiMember.Id)
	config.Email = types.StringValue(apiMember.Email)
	config.Role = types.StringValue(apiMember.Role)
	config.Type = types.StringValue(apiMember.Type)
	config.Status = types.StringValue(apiMember.Status)
	config.Etag = types.StringValue(apiMember.Etag)
	config.DeliverySettings = types.StringValue(apiMember.DeliverySettings)

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
