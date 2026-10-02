package provider

import (
	"context"
	"fmt"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/gmail"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &gmailForwardingAddressDataSource{}

type gmailForwardingAddressDataSourceModel struct {
	UserId             types.String `tfsdk:"user_id"`
	ForwardingEmail    types.String `tfsdk:"forwarding_email"`
	VerificationStatus types.String `tfsdk:"verification_status"`
}

type gmailForwardingAddressDataSource struct {
	client       *gmail.Client
	providerData *gwsProviderData
}

func NewGmailForwardingAddressDataSource() datasource.DataSource {
	return &gmailForwardingAddressDataSource{}
}

func (d *gmailForwardingAddressDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gmail_forwarding_address"
}

func (d *gmailForwardingAddressDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an address a Gmail user may forward mail to.",
		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "The user's email address or the special value \"me\".",
			},
			"forwarding_email": schema.StringAttribute{
				Required:    true,
				Description: "The address mail may be forwarded to.",
			},
			"verification_status": schema.StringAttribute{
				Computed:    true,
				Description: "\"pending\" until the confirmation Gmail sent to the address is acted on, then \"accepted\".",
			},
		},
	}
}

func (d *gmailForwardingAddressDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = data.gmailClient
	d.providerData = data
}

func (d *gmailForwardingAddressDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config gmailForwardingAddressDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := config.UserId.ValueString()
	forwardingEmail := config.ForwardingEmail.ValueString()

	apiAddress, err := d.client.GetForwardingAddress(ctx, userId, forwardingEmail, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Gmail forwarding address",
			fmt.Sprintf("Could not read forwarding address %s for user %s: %s", forwardingEmail, userId, apiErrorDetail(err)),
		)
		return
	}

	if apiAddress == nil {
		resp.Diagnostics.AddError(
			"Gmail forwarding address not found",
			fmt.Sprintf("Forwarding address %s not found for user %s.", forwardingEmail, userId),
		)
		return
	}

	config.ForwardingEmail = types.StringValue(apiAddress.ForwardingEmail)
	config.VerificationStatus = types.StringValue(string(apiAddress.VerificationStatus))

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
