package utils

const BaseImageName = "local_discourse/"

// Known secrets, or otherwise not public info from config so we can build public images
var KnownSecrets = []string{
	"DISCOURSE_DB_HOST",
	"DISCOURSE_DB_PORT",
	"DISCOURSE_DB_REPLICA_HOST",
	"DISCOURSE_DB_REPLICA_PORT",
	"DISCOURSE_DB_PASSWORD",
	"DISCOURSE_REDIS_HOST",
	"DISCOURSE_REDIS_REPLICA_HOST",
	"DISCOURSE_REDIS_PASSWORD",
	"DISCOURSE_SMTP_ADDRESS",
	"DISCOURSE_SMTP_USER_NAME",
	"DISCOURSE_SMTP_PASSWORD",
	"DISCOURSE_DEVELOPER_EMAILS",
	"DISCOURSE_SECRET_KEY_BASE",
	"DISCOURSE_HOSTNAME",
	"DISCOURSE_SAML_CERT",
	"DISCOURSE_SAML_TITLE",
	"DISCOURSE_SAML_TARGET_URL",
	"DISCOURSE_SAML_NAME_IDENTIFIER_FORMAT",
}
