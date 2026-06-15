package provider

import (
	"context"
	"fmt"

	"github.com/Motmedel/utils_go/pkg/cloud/gws/groups_settings"
	groupSettingsType "github.com/Motmedel/utils_go/pkg/cloud/gws/groups_settings/types/group"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &groupSettingsResource{}
	_ resource.ResourceWithImportState = &groupSettingsResource{}
)

type groupSettingsResourceModel struct {
	GroupEmail types.String `tfsdk:"group_email"`

	// Settings
	WhoCanJoin           types.String `tfsdk:"who_can_join"`
	WhoCanViewMembership types.String `tfsdk:"who_can_view_membership"`
	WhoCanViewGroup      types.String `tfsdk:"who_can_view_group"`
	WhoCanPostMessage    types.String `tfsdk:"who_can_post_message"`
	WhoCanLeaveGroup     types.String `tfsdk:"who_can_leave_group"`
	WhoCanContactOwner   types.String `tfsdk:"who_can_contact_owner"`
	WhoCanDiscoverGroup  types.String `tfsdk:"who_can_discover_group"`

	AllowExternalMembers types.String `tfsdk:"allow_external_members"`
	AllowWebPosting      types.String `tfsdk:"allow_web_posting"`
	PrimaryLanguage      types.String `tfsdk:"primary_language"`

	IsArchived  types.String `tfsdk:"is_archived"`
	ArchiveOnly types.String `tfsdk:"archive_only"`

	MessageModerationLevel types.String `tfsdk:"message_moderation_level"`
	SpamModerationLevel    types.String `tfsdk:"spam_moderation_level"`

	ReplyTo                            types.String `tfsdk:"reply_to"`
	CustomReplyTo                      types.String `tfsdk:"custom_reply_to"`
	IncludeCustomFooter                types.String `tfsdk:"include_custom_footer"`
	CustomFooterText                   types.String `tfsdk:"custom_footer_text"`
	SendMessageDenyNotification        types.String `tfsdk:"send_message_deny_notification"`
	DefaultMessageDenyNotificationText types.String `tfsdk:"default_message_deny_notification_text"`

	MembersCanPostAsTheGroup   types.String `tfsdk:"members_can_post_as_the_group"`
	IncludeInGlobalAddressList types.String `tfsdk:"include_in_global_address_list"`
	FavoriteRepliesOnTop       types.String `tfsdk:"favorite_replies_on_top"`

	WhoCanModerateMembers types.String `tfsdk:"who_can_moderate_members"`
	WhoCanModerateContent types.String `tfsdk:"who_can_moderate_content"`
	WhoCanAssistContent   types.String `tfsdk:"who_can_assist_content"`

	EnableCollaborativeInbox types.String `tfsdk:"enable_collaborative_inbox"`
	DefaultSender            types.String `tfsdk:"default_sender"`
}

type groupSettingsResource struct {
	client       *groups_settings.Client
	providerData *gwsProviderData
}

func NewGroupSettingsResource() resource.Resource {
	return &groupSettingsResource{}
}

func (r *groupSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_settings"
}

func (r *groupSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Google Workspace group settings. The group must already exist (use gws_group to create it).",
		Attributes: map[string]schema.Attribute{
			"group_email": schema.StringAttribute{
				Required:    true,
				Description: "The group's email address.",
			},
			"who_can_join": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to join the group. CAN_REQUEST_TO_JOIN, CAN_REQUEST_INVITATION, INVITED_CAN_JOIN, or ALL_IN_DOMAIN_CAN_JOIN.",
			},
			"who_can_view_membership": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to view membership. ALL_IN_DOMAIN_CAN_VIEW, ALL_MEMBERS_CAN_VIEW, or ALL_MANAGERS_CAN_VIEW.",
			},
			"who_can_view_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to view group messages. ANYONE_CAN_VIEW, ALL_IN_DOMAIN_CAN_VIEW, ALL_MEMBERS_CAN_VIEW, or ALL_MANAGERS_CAN_VIEW.",
			},
			"who_can_post_message": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to post messages. NONE_CAN_POST, ALL_MANAGERS_CAN_POST, ALL_MEMBERS_CAN_POST, ALL_OWNERS_CAN_POST, ALL_IN_DOMAIN_CAN_POST, or ANYONE_CAN_POST.",
			},
			"who_can_leave_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to leave the group. ALL_MANAGERS_CAN_LEAVE, ALL_MEMBERS_CAN_LEAVE, or NONE_CAN_LEAVE.",
			},
			"who_can_contact_owner": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to contact the group owner. ALL_IN_DOMAIN_CAN_CONTACT, ALL_MANAGERS_CAN_CONTACT, ALL_MEMBERS_CAN_CONTACT, or ANYONE_CAN_CONTACT.",
			},
			"who_can_discover_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to discover the group. ANYONE_CAN_DISCOVER, ALL_IN_DOMAIN_CAN_DISCOVER, or ALL_MEMBERS_CAN_DISCOVER.",
			},
			"allow_external_members": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether external members can be added to the group. true or false.",
			},
			"allow_web_posting": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether posting from web is allowed. true or false.",
			},
			"primary_language": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The primary language for the group.",
			},
			"is_archived": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the group is archived. true or false.",
			},
			"archive_only": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the group is archive-only. true or false.",
			},
			"message_moderation_level": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Moderation level for messages. MODERATE_ALL_MESSAGES, MODERATE_NON_MEMBERS, MODERATE_NEW_MEMBERS, or MODERATE_NONE.",
			},
			"spam_moderation_level": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Spam moderation level. ALLOW, MODERATE, SILENTLY_MODERATE, or REJECT.",
			},
			"reply_to": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Reply-to setting. REPLY_TO_CUSTOM, REPLY_TO_SENDER, REPLY_TO_LIST, REPLY_TO_OWNER, REPLY_TO_IGNORE, or REPLY_TO_MANAGERS.",
			},
			"custom_reply_to": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Custom email address used when reply_to is REPLY_TO_CUSTOM.",
			},
			"include_custom_footer": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to include a custom footer. true or false.",
			},
			"custom_footer_text": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Custom footer text (max 1000 characters).",
			},
			"send_message_deny_notification": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to send a notification when a message is denied. true or false.",
			},
			"default_message_deny_notification_text": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Default message shown when a post is denied.",
			},
			"members_can_post_as_the_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether members can post as the group. true or false.",
			},
			"include_in_global_address_list": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the group appears in the Global Address List. true or false.",
			},
			"favorite_replies_on_top": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether favorite replies are shown first. true or false.",
			},
			"who_can_moderate_members": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to moderate members. ALL_MEMBERS, OWNERS_AND_MANAGERS, OWNERS_ONLY, or NONE.",
			},
			"who_can_moderate_content": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to moderate content. ALL_MEMBERS, OWNERS_AND_MANAGERS, OWNERS_ONLY, or NONE.",
			},
			"who_can_assist_content": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Permission to assist content. ALL_MEMBERS, OWNERS_AND_MANAGERS, OWNERS_ONLY, or NONE.",
			},
			"enable_collaborative_inbox": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether collaborative inbox is enabled. true or false.",
			},
			"default_sender": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Default sender when posting as group. DEFAULT_SELF or GROUP.",
			},
		},
	}
}

func (r *groupSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := getProviderData(req.ProviderData, &resp.Diagnostics)
	if data == nil {
		return
	}
	r.client = data.groupsSettingsClient
	r.providerData = data
}

func (r *groupSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupEmail := plan.GroupEmail.ValueString()

	// Group settings always exist when the group exists. "Create" means setting desired values.
	apiSettings := buildGroupSettingsFromPlan(&plan)

	updatedSettings, err := r.client.Patch(ctx, groupEmail, apiSettings, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error setting group settings",
			fmt.Sprintf("Could not set settings for group %s: %s", groupEmail, apiErrorDetail(err)),
		)
		return
	}

	mapGroupSettingsToState(updatedSettings, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupEmail := state.GroupEmail.ValueString()

	apiSettings, err := r.client.Get(ctx, groupEmail, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading group settings",
			fmt.Sprintf("Could not read settings for group %s: %s", groupEmail, apiErrorDetail(err)),
		)
		return
	}

	if apiSettings == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapGroupSettingsToState(apiSettings, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *groupSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupEmail := plan.GroupEmail.ValueString()
	apiSettings := buildGroupSettingsFromPlan(&plan)

	updatedSettings, err := r.client.Patch(ctx, groupEmail, apiSettings, fetchOptions(r.providerData)...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating group settings",
			fmt.Sprintf("Could not update settings for group %s: %s", groupEmail, apiErrorDetail(err)),
		)
		return
	}

	mapGroupSettingsToState(updatedSettings, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Group settings can't be deleted independently — they're tied to the group.
	// Removing from state is sufficient.
}

func (r *groupSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("group_email"), req, resp)
}

func buildGroupSettingsFromPlan(plan *groupSettingsResourceModel) *groupSettingsType.Group {
	g := &groupSettingsType.Group{}

	if !plan.WhoCanJoin.IsNull() && !plan.WhoCanJoin.IsUnknown() {
		g.WhoCanJoin = plan.WhoCanJoin.ValueString()
	}
	if !plan.WhoCanViewMembership.IsNull() && !plan.WhoCanViewMembership.IsUnknown() {
		g.WhoCanViewMembership = plan.WhoCanViewMembership.ValueString()
	}
	if !plan.WhoCanViewGroup.IsNull() && !plan.WhoCanViewGroup.IsUnknown() {
		g.WhoCanViewGroup = plan.WhoCanViewGroup.ValueString()
	}
	if !plan.WhoCanPostMessage.IsNull() && !plan.WhoCanPostMessage.IsUnknown() {
		g.WhoCanPostMessage = plan.WhoCanPostMessage.ValueString()
	}
	if !plan.WhoCanLeaveGroup.IsNull() && !plan.WhoCanLeaveGroup.IsUnknown() {
		g.WhoCanLeaveGroup = plan.WhoCanLeaveGroup.ValueString()
	}
	if !plan.WhoCanContactOwner.IsNull() && !plan.WhoCanContactOwner.IsUnknown() {
		g.WhoCanContactOwner = plan.WhoCanContactOwner.ValueString()
	}
	if !plan.WhoCanDiscoverGroup.IsNull() && !plan.WhoCanDiscoverGroup.IsUnknown() {
		g.WhoCanDiscoverGroup = plan.WhoCanDiscoverGroup.ValueString()
	}
	if !plan.AllowExternalMembers.IsNull() && !plan.AllowExternalMembers.IsUnknown() {
		g.AllowExternalMembers = plan.AllowExternalMembers.ValueString()
	}
	if !plan.AllowWebPosting.IsNull() && !plan.AllowWebPosting.IsUnknown() {
		g.AllowWebPosting = plan.AllowWebPosting.ValueString()
	}
	if !plan.PrimaryLanguage.IsNull() && !plan.PrimaryLanguage.IsUnknown() {
		g.PrimaryLanguage = plan.PrimaryLanguage.ValueString()
	}
	if !plan.IsArchived.IsNull() && !plan.IsArchived.IsUnknown() {
		g.IsArchived = plan.IsArchived.ValueString()
	}
	if !plan.ArchiveOnly.IsNull() && !plan.ArchiveOnly.IsUnknown() {
		g.ArchiveOnly = plan.ArchiveOnly.ValueString()
	}
	if !plan.MessageModerationLevel.IsNull() && !plan.MessageModerationLevel.IsUnknown() {
		g.MessageModerationLevel = plan.MessageModerationLevel.ValueString()
	}
	if !plan.SpamModerationLevel.IsNull() && !plan.SpamModerationLevel.IsUnknown() {
		g.SpamModerationLevel = plan.SpamModerationLevel.ValueString()
	}
	if !plan.ReplyTo.IsNull() && !plan.ReplyTo.IsUnknown() {
		g.ReplyTo = plan.ReplyTo.ValueString()
	}
	if !plan.CustomReplyTo.IsNull() && !plan.CustomReplyTo.IsUnknown() {
		g.CustomReplyTo = plan.CustomReplyTo.ValueString()
	}
	if !plan.IncludeCustomFooter.IsNull() && !plan.IncludeCustomFooter.IsUnknown() {
		g.IncludeCustomFooter = plan.IncludeCustomFooter.ValueString()
	}
	if !plan.CustomFooterText.IsNull() && !plan.CustomFooterText.IsUnknown() {
		g.CustomFooterText = plan.CustomFooterText.ValueString()
	}
	if !plan.SendMessageDenyNotification.IsNull() && !plan.SendMessageDenyNotification.IsUnknown() {
		g.SendMessageDenyNotification = plan.SendMessageDenyNotification.ValueString()
	}
	if !plan.DefaultMessageDenyNotificationText.IsNull() && !plan.DefaultMessageDenyNotificationText.IsUnknown() {
		g.DefaultMessageDenyNotificationText = plan.DefaultMessageDenyNotificationText.ValueString()
	}
	if !plan.MembersCanPostAsTheGroup.IsNull() && !plan.MembersCanPostAsTheGroup.IsUnknown() {
		g.MembersCanPostAsTheGroup = plan.MembersCanPostAsTheGroup.ValueString()
	}
	if !plan.IncludeInGlobalAddressList.IsNull() && !plan.IncludeInGlobalAddressList.IsUnknown() {
		g.IncludeInGlobalAddressList = plan.IncludeInGlobalAddressList.ValueString()
	}
	if !plan.FavoriteRepliesOnTop.IsNull() && !plan.FavoriteRepliesOnTop.IsUnknown() {
		g.FavoriteRepliesOnTop = plan.FavoriteRepliesOnTop.ValueString()
	}
	if !plan.WhoCanModerateMembers.IsNull() && !plan.WhoCanModerateMembers.IsUnknown() {
		g.WhoCanModerateMembers = plan.WhoCanModerateMembers.ValueString()
	}
	if !plan.WhoCanModerateContent.IsNull() && !plan.WhoCanModerateContent.IsUnknown() {
		g.WhoCanModerateContent = plan.WhoCanModerateContent.ValueString()
	}
	if !plan.WhoCanAssistContent.IsNull() && !plan.WhoCanAssistContent.IsUnknown() {
		g.WhoCanAssistContent = plan.WhoCanAssistContent.ValueString()
	}
	if !plan.EnableCollaborativeInbox.IsNull() && !plan.EnableCollaborativeInbox.IsUnknown() {
		g.EnableCollaborativeInbox = plan.EnableCollaborativeInbox.ValueString()
	}
	if !plan.DefaultSender.IsNull() && !plan.DefaultSender.IsUnknown() {
		g.DefaultSender = plan.DefaultSender.ValueString()
	}

	return g
}

func mapGroupSettingsToState(g *groupSettingsType.Group, state *groupSettingsResourceModel) {
	if g == nil {
		return
	}

	if g.Email != "" {
		state.GroupEmail = types.StringValue(g.Email)
	}

	state.WhoCanJoin = types.StringValue(g.WhoCanJoin)
	state.WhoCanViewMembership = types.StringValue(g.WhoCanViewMembership)
	state.WhoCanViewGroup = types.StringValue(g.WhoCanViewGroup)
	state.WhoCanPostMessage = types.StringValue(g.WhoCanPostMessage)
	state.WhoCanLeaveGroup = types.StringValue(g.WhoCanLeaveGroup)
	state.WhoCanContactOwner = types.StringValue(g.WhoCanContactOwner)
	state.WhoCanDiscoverGroup = types.StringValue(g.WhoCanDiscoverGroup)
	state.AllowExternalMembers = types.StringValue(g.AllowExternalMembers)
	state.AllowWebPosting = types.StringValue(g.AllowWebPosting)
	state.PrimaryLanguage = types.StringValue(g.PrimaryLanguage)
	state.IsArchived = types.StringValue(g.IsArchived)
	state.ArchiveOnly = types.StringValue(g.ArchiveOnly)
	state.MessageModerationLevel = types.StringValue(g.MessageModerationLevel)
	state.SpamModerationLevel = types.StringValue(g.SpamModerationLevel)
	state.ReplyTo = types.StringValue(g.ReplyTo)
	state.CustomReplyTo = types.StringValue(g.CustomReplyTo)
	state.IncludeCustomFooter = types.StringValue(g.IncludeCustomFooter)
	state.CustomFooterText = types.StringValue(g.CustomFooterText)
	state.SendMessageDenyNotification = types.StringValue(g.SendMessageDenyNotification)
	state.DefaultMessageDenyNotificationText = types.StringValue(g.DefaultMessageDenyNotificationText)
	state.MembersCanPostAsTheGroup = types.StringValue(g.MembersCanPostAsTheGroup)
	state.IncludeInGlobalAddressList = types.StringValue(g.IncludeInGlobalAddressList)
	state.FavoriteRepliesOnTop = types.StringValue(g.FavoriteRepliesOnTop)
	state.WhoCanModerateMembers = types.StringValue(g.WhoCanModerateMembers)
	state.WhoCanModerateContent = types.StringValue(g.WhoCanModerateContent)
	state.WhoCanAssistContent = types.StringValue(g.WhoCanAssistContent)
	state.EnableCollaborativeInbox = types.StringValue(g.EnableCollaborativeInbox)
	state.DefaultSender = types.StringValue(g.DefaultSender)
}
