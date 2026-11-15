package provider

import (
	"context"
	"fmt"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/directory"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &usersDataSource{}

type usersDataSourceModel struct {
	Customer types.String          `tfsdk:"customer"`
	Users    []userDataSourceModel `tfsdk:"users"`
}

type usersDataSource struct {
	client       *directory.Client
	providerData *gwsProviderData
}

func NewUsersDataSource() datasource.DataSource {
	return &usersDataSource{}
}

func (d *usersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *usersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all Google Workspace users.",
		Attributes: map[string]schema.Attribute{
			"customer": schema.StringAttribute{
				Optional:    true,
				Description: "Customer ID. Defaults to \"my_customer\".",
			},
			"users": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of users.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: userDataSourceAttributes(false),
				},
			},
		},
	}
}

func (d *usersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *usersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config usersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	customer := config.Customer.ValueString()
	if customer == "" {
		customer = "my_customer"
	}

	users, err := d.client.ListUsers(ctx, customer, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing users",
			fmt.Sprintf("Could not list users: %s", apiErrorDetail(err)),
		)
		return
	}

	var entries []userDataSourceModel
	for _, u := range users {
		if u == nil {
			continue
		}
		entries = append(entries, newUserDataSourceModel(u))
	}

	config.Users = entries
	config.Customer = types.StringValue(customer)
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
