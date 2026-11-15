package provider

import (
	"context"
	"fmt"

	"github.com/altshiftab/utils_go/pkg/cloud/gws/groups_settings"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var _ datasource.DataSource = &groupSettingsDataSource{}

type groupSettingsDataSource struct {
	client       *groups_settings.Client
	providerData *gwsProviderData
}

func NewGroupSettingsDataSource() datasource.DataSource {
	return &groupSettingsDataSource{}
}

func (d *groupSettingsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_settings"
}

func (d *groupSettingsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches Google Workspace group settings.",
		Attributes: map[string]schema.Attribute{
			"group_email": schema.StringAttribute{
				Required:    true,
				Description: "The group's email address.",
			},
			"who_can_join": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to join the group.",
			},
			"who_can_view_membership": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to view membership.",
			},
			"who_can_view_group": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to view group messages.",
			},
			"who_can_post_message": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to post messages.",
			},
			"who_can_leave_group": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to leave the group.",
			},
			"who_can_contact_owner": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to contact the group owner.",
			},
			"who_can_discover_group": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to discover the group.",
			},
			"allow_external_members": schema.StringAttribute{
				Computed:    true,
				Description: "Whether external members can be added to the group.",
			},
			"allow_web_posting": schema.StringAttribute{
				Computed:    true,
				Description: "Whether posting from web is allowed.",
			},
			"primary_language": schema.StringAttribute{
				Computed:    true,
				Description: "The primary language for the group.",
			},
			"is_archived": schema.StringAttribute{
				Computed:    true,
				Description: "Whether the group is archived.",
			},
			"archive_only": schema.StringAttribute{
				Computed:    true,
				Description: "Whether the group is archive-only.",
			},
			"message_moderation_level": schema.StringAttribute{
				Computed:    true,
				Description: "Moderation level for messages.",
			},
			"spam_moderation_level": schema.StringAttribute{
				Computed:    true,
				Description: "Spam moderation level.",
			},
			"reply_to": schema.StringAttribute{
				Computed:    true,
				Description: "Reply-to setting.",
			},
			"custom_reply_to": schema.StringAttribute{
				Computed:    true,
				Description: "Custom email address used when reply_to is REPLY_TO_CUSTOM.",
			},
			"include_custom_footer": schema.StringAttribute{
				Computed:    true,
				Description: "Whether to include a custom footer.",
			},
			"custom_footer_text": schema.StringAttribute{
				Computed:    true,
				Description: "Custom footer text.",
			},
			"send_message_deny_notification": schema.StringAttribute{
				Computed:    true,
				Description: "Whether to send a notification when a message is denied.",
			},
			"default_message_deny_notification_text": schema.StringAttribute{
				Computed:    true,
				Description: "Default message shown when a post is denied.",
			},
			"members_can_post_as_the_group": schema.StringAttribute{
				Computed:    true,
				Description: "Whether members can post as the group.",
			},
			"include_in_global_address_list": schema.StringAttribute{
				Computed:    true,
				Description: "Whether the group appears in the Global Address List.",
			},
			"favorite_replies_on_top": schema.StringAttribute{
				Computed:    true,
				Description: "Whether favorite replies are shown first.",
			},
			"who_can_moderate_members": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to moderate members.",
			},
			"who_can_moderate_content": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to moderate content.",
			},
			"who_can_assist_content": schema.StringAttribute{
				Computed:    true,
				Description: "Permission to assist content.",
			},
			"enable_collaborative_inbox": schema.StringAttribute{
				Computed:    true,
				Description: "Whether collaborative inbox is enabled.",
			},
			"default_sender": schema.StringAttribute{
				Computed:    true,
				Description: "Default sender when posting as group.",
			},
		},
	}
}

func (d *groupSettingsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = data.groupsSettingsClient
	d.providerData = data
}

func (d *groupSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config groupSettingsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupEmail := config.GroupEmail.ValueString()

	apiSettings, err := d.client.Get(ctx, groupEmail, fetchOptions(d.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading group settings",
			fmt.Sprintf("Could not read settings for group %s: %s", groupEmail, apiErrorDetail(err)),
		)
		return
	}

	if apiSettings == nil {
		resp.Diagnostics.AddError(
			"Group settings not found",
			fmt.Sprintf("Settings for group %s not found.", groupEmail),
		)
		return
	}

	mapGroupSettingsToState(apiSettings, &config)
	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}
