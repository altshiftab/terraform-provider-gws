package provider

// Attribute names and descriptions that several resources and data sources share.
const (
	emailAttribute         = "email"
	etagDescription        = "ETag of the resource."
	groupEmailDescription  = "The group's email address."
	groupKeyAttribute      = "group_key"
	groupKeyDescription    = "The group's email address or unique ID."
	gmailUserIdAttribute   = "user_id"
	gmailUserIdDescription = `The user's email address or the special value "me".`
	// filterSubjectAttribute is the Gmail filter criterion, not the provider's impersonated subject.
	filterSubjectAttribute              = "subject"
	includeInGlobalAddressListAttribute = "include_in_global_address_list"
	verificationStatusAttribute         = "verification_status"
)
