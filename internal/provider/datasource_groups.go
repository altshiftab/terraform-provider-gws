package provider

import (
	"context"
	"fmt"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/directory"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &groupsDataSource{}

type groupsDataSourceModel struct {
	Customer types.String      `tfsdk:"customer"`
	Groups   []groupEntryModel `tfsdk:"groups"`
}

type groupEntryModel struct {
	Id                 types.String `tfsdk:"id"`
	Email              types.String `tfsdk:"email"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	DirectMembersCount types.String `tfsdk:"direct_members_count"`
	AdminCreated       types.Bool   `tfsdk:"admin_created"`
}

type groupsDataSource struct {
	client       *directory.Client
	providerData *gwsProviderData
}

func NewGroupsDataSource() datasource.DataSource {
	return &groupsDataSource{}
}

func (d *groupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_groups"
}

func (d *groupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all Google Workspace groups.",
		Attributes: map[string]schema.Attribute{
			"customer": schema.StringAttribute{
				Optional:    true,
				Description: "Customer ID. Defaults to \"my_customer\".",
			},
			"groups": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of groups.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"email": schema.StringAttribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
						"description": schema.StringAttribute{
							Computed: true,
						},
						"direct_members_count": schema.StringAttribute{
							Computed: true,
						},
						"admin_created": schema.BoolAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (d *groupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *groupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config groupsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	customer := config.Customer.ValueString()
	if customer == "" {
		customer = "my_customer"
	}

	groups, err := d.client.ListGroups(ctx, customer, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing groups",
			fmt.Sprintf("Could not list groups: %s", err),
		)
		return
	}

	var entries []groupEntryModel
	for _, g := range groups {
		entry := groupEntryModel{
			Id:                 types.StringValue(g.Id),
			Email:              types.StringValue(g.Email),
			Name:               types.StringValue(g.Name),
			DirectMembersCount: types.StringValue(g.DirectMembersCount),
			AdminCreated:       types.BoolValue(g.AdminCreated),
		}

		if g.Description != "" {
			entry.Description = types.StringValue(g.Description)
		} else {
			entry.Description = types.StringNull()
		}

		entries = append(entries, entry)
	}

	config.Groups = entries
	config.Customer = types.StringValue(customer)
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
