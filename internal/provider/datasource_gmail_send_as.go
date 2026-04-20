package provider

import (
	"context"
	"fmt"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/gmail"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &gmailSendAsDataSource{}

type gmailSendAsDataSourceModel struct {
	UserId             types.String `tfsdk:"user_id"`
	SendAsEmail        types.String `tfsdk:"send_as_email"`
	DisplayName        types.String `tfsdk:"display_name"`
	ReplyToAddress     types.String `tfsdk:"reply_to_address"`
	Signature          types.String `tfsdk:"signature"`
	TreatAsAlias       types.Bool   `tfsdk:"treat_as_alias"`
	IsPrimary          types.Bool   `tfsdk:"is_primary"`
	IsDefault          types.Bool   `tfsdk:"is_default"`
	VerificationStatus types.String `tfsdk:"verification_status"`
}

type gmailSendAsDataSource struct {
	client       *gmail.Client
	providerData *gwsProviderData
}

func NewGmailSendAsDataSource() datasource.DataSource {
	return &gmailSendAsDataSource{}
}

func (d *gmailSendAsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gmail_send_as"
}

func (d *gmailSendAsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Gmail send-as alias for a Google Workspace user.",
		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "The user's email address or the special value \"me\".",
			},
			"send_as_email": schema.StringAttribute{
				Required:    true,
				Description: "The email address that appears in the \"From:\" header for mail sent using this alias.",
			},
			"display_name": schema.StringAttribute{
				Computed:    true,
				Description: "A name that appears in the \"From:\" header for mail sent using this alias.",
			},
			"reply_to_address": schema.StringAttribute{
				Computed:    true,
				Description: "An optional email address that is included in a \"Reply-To:\" header for mail sent using this alias.",
			},
			"signature": schema.StringAttribute{
				Computed:    true,
				Description: "An optional HTML signature that is included in messages composed with this alias in the Gmail web UI.",
			},
			"treat_as_alias": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether Gmail should treat this address as an alias for the user's primary email address.",
			},
			"is_primary": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this address is the primary address used to login to the account.",
			},
			"is_default": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this address is selected as the default \"From:\" address.",
			},
			"verification_status": schema.StringAttribute{
				Computed:    true,
				Description: "Indicates whether this address has been verified for use as a send-as alias.",
			},
		},
	}
}

func (d *gmailSendAsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *gmailSendAsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config gmailSendAsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := config.UserId.ValueString()
	sendAsEmail := config.SendAsEmail.ValueString()

	apiSendAs, err := d.client.GetSendAs(ctx, userId, sendAsEmail, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Gmail send-as alias",
			fmt.Sprintf("Could not read send-as alias %s for user %s: %s", sendAsEmail, userId, err),
		)
		return
	}

	if apiSendAs == nil {
		resp.Diagnostics.AddError(
			"Gmail send-as alias not found",
			fmt.Sprintf("Send-as alias %s not found for user %s.", sendAsEmail, userId),
		)
		return
	}

	config.SendAsEmail = types.StringValue(apiSendAs.SendAsEmail)
	config.DisplayName = types.StringValue(apiSendAs.DisplayName)
	config.ReplyToAddress = types.StringValue(apiSendAs.ReplyToAddress)
	config.Signature = types.StringValue(apiSendAs.Signature)
	config.TreatAsAlias = types.BoolValue(apiSendAs.TreatAsAlias)
	config.IsPrimary = types.BoolValue(apiSendAs.IsPrimary)
	config.IsDefault = types.BoolValue(apiSendAs.IsDefault)
	config.VerificationStatus = types.StringValue(apiSendAs.VerificationStatus)

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
