package provider

import (
	"context"
	"fmt"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &groupMembersDataSource{}

type groupMembersDataSourceModel struct {
	GroupKey types.String       `tfsdk:"group_key"`
	Members  []memberEntryModel `tfsdk:"members"`
}

type memberEntryModel struct {
	Id               types.String `tfsdk:"id"`
	Email            types.String `tfsdk:"email"`
	Role             types.String `tfsdk:"role"`
	Type             types.String `tfsdk:"type"`
	Status           types.String `tfsdk:"status"`
	Etag             types.String `tfsdk:"etag"`
	DeliverySettings types.String `tfsdk:"delivery_settings"`
}

type groupMembersDataSource struct {
	client       *directory.Client
	providerData *gwsProviderData
}

func NewGroupMembersDataSource() datasource.DataSource {
	return &groupMembersDataSource{}
}

func (d *groupMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_members"
}

func (d *groupMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all members of a Google Workspace group.",
		Attributes: map[string]schema.Attribute{
			"group_key": schema.StringAttribute{
				Required:    true,
				Description: "The group's email address or unique ID.",
			},
			"members": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of group members.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The unique ID of the group member.",
						},
						"email": schema.StringAttribute{
							Computed:    true,
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
						"status": schema.StringAttribute{
							Computed:    true,
							Description: "The status of the member.",
						},
						"etag": schema.StringAttribute{
							Computed:    true,
							Description: "ETag of the resource.",
						},
						"delivery_settings": schema.StringAttribute{
							Computed:    true,
							Description: "Delivery settings for the member. ALL_MAIL, DAILY, DIGEST, DISABLED, or NONE.",
						},
					},
				},
			},
		},
	}
}

func (d *groupMembersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *groupMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config groupMembersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupKey := config.GroupKey.ValueString()

	members, err := d.client.ListMembers(ctx, groupKey, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing group members",
			fmt.Sprintf("Could not list members of group %s: %s", groupKey, err),
		)
		return
	}

	var entries []memberEntryModel
	for _, m := range members {
		entries = append(entries, memberEntryModel{
			Id:               types.StringValue(m.Id),
			Email:            types.StringValue(m.Email),
			Role:             types.StringValue(m.Role),
			Type:             types.StringValue(m.Type),
			Status:           types.StringValue(m.Status),
			Etag:             types.StringValue(m.Etag),
			DeliverySettings: types.StringValue(m.DeliverySettings),
		})
	}

	config.Members = entries
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
