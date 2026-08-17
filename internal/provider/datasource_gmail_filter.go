package provider

import (
	"context"
	"fmt"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/gmail"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &gmailFilterDataSource{}

type gmailFilterDataSourceModel struct {
	UserId   types.String              `tfsdk:"user_id"`
	FilterId types.String              `tfsdk:"filter_id"`
	Criteria *gmailFilterCriteriaModel `tfsdk:"criteria"`
	Action   *gmailFilterActionModel   `tfsdk:"action"`
}

type gmailFilterDataSource struct {
	client       *gmail.Client
	providerData *gwsProviderData
}

func NewGmailFilterDataSource() datasource.DataSource {
	return &gmailFilterDataSource{}
}

func (d *gmailFilterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gmail_filter"
}

func (d *gmailFilterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Gmail filter for a Google Workspace user.",
		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "The user's email address or the special value \"me\".",
			},
			"filter_id": schema.StringAttribute{
				Required:    true,
				Description: "The server-assigned identifier of the filter.",
			},
			"criteria": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Matching criteria for the filter.",
				Attributes: map[string]schema.Attribute{
					"from": schema.StringAttribute{
						Computed:    true,
						Description: "The sender's display name or email address.",
					},
					"to": schema.StringAttribute{
						Computed:    true,
						Description: "The recipient's display name or email address.",
					},
					"subject": schema.StringAttribute{
						Computed:    true,
						Description: "Case-insensitive phrase found in the message's subject.",
					},
					"query": schema.StringAttribute{
						Computed:    true,
						Description: "Gmail search query.",
					},
					"negated_query": schema.StringAttribute{
						Computed:    true,
						Description: "Gmail search query that must NOT match.",
					},
					"has_attachment": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether the message has any attachment.",
					},
					"exclude_chats": schema.BoolAttribute{
						Computed:    true,
						Description: "Whether chat messages should be excluded.",
					},
					"size": schema.Int64Attribute{
						Computed:    true,
						Description: "Size of the entire RFC822 message in bytes.",
					},
					"size_comparison": schema.StringAttribute{
						Computed:    true,
						Description: "How the message size is compared.",
					},
				},
			},
			"action": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Action to apply to matching messages.",
				Attributes: map[string]schema.Attribute{
					"add_label_ids": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
						Description: "Labels to add to matching messages.",
					},
					"remove_label_ids": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
						Description: "Labels to remove from matching messages.",
					},
					"forward": schema.StringAttribute{
						Computed:    true,
						Description: "Email address to forward matching messages to.",
					},
				},
			},
		},
	}
}

func (d *gmailFilterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *gmailFilterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config gmailFilterDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userId := config.UserId.ValueString()
	filterId := config.FilterId.ValueString()

	apiFilter, err := d.client.GetFilter(ctx, userId, filterId, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Gmail filter",
			fmt.Sprintf("Could not read filter %s for user %s: %s", filterId, userId, apiErrorDetail(err)),
		)
		return
	}

	if apiFilter == nil {
		resp.Diagnostics.AddError(
			"Gmail filter not found",
			fmt.Sprintf("Filter %s not found for user %s.", filterId, userId),
		)
		return
	}

	config.FilterId = types.StringValue(apiFilter.Id)
	config.Criteria = mapCriteriaToModel(apiFilter.Criteria)
	action, diags := mapActionToModel(ctx, apiFilter.Action)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Action = action

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
