package schemes

// Error codes are part of the API contract.
// Rules:
// - Codes are stable identifiers for frontend/business logic.
// - HTTP status is determined by ErrKind (see ErrorResponse.Kind), not by Code.
// - Codes are grouped by domain (auth/workspaces/invites/oauth/telegram/internal).

// --- 1xxx: Auth / Tokens ---
const (
	// ErrAuthInvalidAccessToken — access token is missing/invalid/expired.
	ErrAuthInvalidAccessToken = 1101
	// ErrAuthInvalidTicket — login ticket is invalid/expired/invalid signature.
	ErrAuthInvalidTicket = 1102
	// ErrAuthTicketSignFailed — server couldn't sign ticket (internal).
	ErrAuthTicketSignFailed = 1103
	// ErrAuthTokenMissing — access token missing in request.
	ErrAuthTokenMissing = 1104
	// ErrAuthTicketDomainMismatch — domain mismatch in login-ticket.
	ErrAuthTicketDomainMismatch = 1105
	// ErrAuthTicketDeviceMismatch — device mismatch in login-ticket (state protection).
	ErrAuthTicketDeviceMismatch = 1106

	// ErrAuthInvalidRefreshToken — refresh token failed validation (signature/claims).
	ErrAuthInvalidRefreshToken = 1201
	// ErrAuthRefreshNotFound — refresh token hash not found in sessions.
	ErrAuthRefreshNotFound = 1202
	// ErrAuthRefreshExpiredOrReused — session expired/inactive/reused token.
	ErrAuthRefreshExpiredOrReused = 1203
	// ErrAuthDeviceMismatch — device_id mismatch for token/session operation.
	ErrAuthDeviceMismatch = 1204
	// ErrAuthSessionDomainMismatch — session/token belongs to another domain.
	ErrAuthSessionDomainMismatch = 1205
	// ErrAuthSessionExpiredOrInactive — refresh session expired/inactive.
	ErrAuthSessionExpiredOrInactive = 1206
	// ErrAuthInvalidCredentials — invalid login/password or similar.
	ErrAuthInvalidCredentials = 1301
)

// --- 2xxx: workspaces / Members ---
const (
	// ErrWorkspaceInvalidID — invalid workspace_id format/value.
	ErrWorkspaceInvalidID = 2001
	// ErrWorkspaceInvalidTargetUserID — invalid target_user_id format/value.
	ErrWorkspaceInvalidTargetUserID = 2000
	// ErrWorkspaceInvalidRole — invalid role value.
	ErrWorkspaceInvalidRole = 2020
	// ErrWorkspaceNotFound — workspace not found.
	ErrWorkspaceNotFound = 2002
	// ErrWorkspaceDomainMismatch — workspace belongs to another domain.
	ErrWorkspaceDomainMismatch = 2003
	// ErrWorkspaceNotMember — user isn't a member of the workspace.
	ErrWorkspaceNotMember = 2004
	// ErrWorkspaceInsufficientPermissions — role doesn't have enough permissions.
	ErrWorkspaceInsufficientPermissions = 2005
	// ErrWorkspaceMemberLimitReached — workspace member limit reached.
	ErrWorkspaceMemberLimitReached = 2006
	// ErrWorkspaceUserAlreadyMember — user already in workspace.
	ErrWorkspaceUserAlreadyMember = 2007
	// ErrWorkspaceTargetUserNotFound — target user not found.
	ErrWorkspaceTargetUserNotFound = 2008
	// ErrWorkspaceOwnerOnlyAction — action requires owner role.
	ErrWorkspaceOwnerOnlyAction = 2009
	// ErrWorkspaceMemberNotFound — membership record not found.
	ErrWorkspaceMemberNotFound = 2010
	// ErrWorkspaceOwnerRoleImmutable — cannot change/assign owner role via this endpoint.
	ErrWorkspaceOwnerRoleImmutable = 2011
	// ErrworkspacesingleMemberOnly — service/workspace is single-member, cannot add more users.
	ErrworkspacesingleMemberOnly = 2012
	// ErrWorkspaceVirtualLoginAlreadyExists — virtual login already exists.
	ErrWorkspaceVirtualLoginAlreadyExists = 2013
	// ErrWorkspaceTargetUserNotVirtual — target user is not virtual.
	ErrWorkspaceTargetUserNotVirtual = 2014
	// ErrWorkspaceCannotDeleteOwner — cannot delete workspace owner.
	ErrWorkspaceCannotDeleteOwner = 2015
	// ErrWorkspaceCreationDisabledSingleWorkspace — workspace creation disabled for single-workspace services.
	ErrWorkspaceCreationDisabledSingleWorkspace = 2016
	// ErrWorkspaceDeletionDisabledSingleWorkspace — workspace deletion disabled for single-workspace services.
	ErrWorkspaceDeletionDisabledSingleWorkspace = 2017
	// ErrWorkspaceOnlyOwnerCanDelete — only owner can delete workspace.
	ErrWorkspaceOnlyOwnerCanDelete = 2018
	// ErrWorkspaceCannotDeleteLastWorkspace — cannot delete last remaining workspace.
	ErrWorkspaceCannotDeleteLastWorkspace = 2019
)

// --- 3xxx: Invites ---
const (
	// ErrInviteNotFound — invite token not found/unknown.
	ErrInviteNotFound = 3001
	// ErrInviteAlreadyUsed — invite token already used.
	ErrInviteAlreadyUsed = 3002
	// ErrInviteExpired — invite token expired (use HTTP 410 Gone).
	ErrInviteExpired = 3003
	// ErrInviteAuthMethodMismatch — invite restricted to a different auth_method.
	ErrInviteAuthMethodMismatch = 3004
	// ErrInviteOAuthProviderMismatch — invite restricted to a different oauth2_provider.
	ErrInviteOAuthProviderMismatch = 3005
	// ErrInviteRoleNotAllowed — invite role is not allowed (e.g. owner).
	ErrInviteRoleNotAllowed = 3006
	// ErrInviteAuthMethodNotAllowed — invite auth_method is not allowed for this domain.
	ErrInviteAuthMethodNotAllowed = 3007
	// ErrInviteOAuthProviderRequiresOAuth2 — oauth2_provider requires auth_method=oauth2.
	ErrInviteOAuthProviderRequiresOAuth2 = 3008
	// ErrInviteOAuthProviderUnsupported — unsupported oauth2_provider for this domain.
	ErrInviteOAuthProviderUnsupported = 3009
)

// --- 4xxx: OAuth2 / Telegram ---
const (
	// ErrOAuthInvalidState — invalid oauth2 state (bad signature/expired).
	ErrOAuthInvalidState = 4001
	// ErrOAuthDomainMismatch — domain mismatch between ctx and state/callback.
	ErrOAuthDomainMismatch = 4002
	// ErrOAuthUnsupportedProvider — provider not configured/unsupported.
	ErrOAuthUnsupportedProvider = 4003
	// ErrOAuthCodeExchangeFailed — failed to exchange code for tokens at provider.
	ErrOAuthCodeExchangeFailed = 4004
	// ErrOAuthUserInfoFetchFailed — failed to fetch userinfo from provider (network/upstream).
	ErrOAuthUserInfoFetchFailed = 4005
	// ErrOAuthUserInfoNon2xx — provider userinfo returned non-2xx.
	ErrOAuthUserInfoNon2xx = 4006
	// ErrOAuthUserInfoJSONDecodeFailed — JSON decode failed for provider userinfo.
	ErrOAuthUserInfoJSONDecodeFailed = 4007
	// ErrOAuthUserInfoMissingID — provider userinfo missing required id.
	ErrOAuthUserInfoMissingID = 4008

	// ErrTelegramInvalidHash — invalid telegram login hash.
	ErrTelegramInvalidHash = 4101
	// ErrTelegramAuthDateFuture — auth_date in the future.
	ErrTelegramAuthDateFuture = 4102
	// ErrTelegramAuthDateTooOld — auth_date too old.
	ErrTelegramAuthDateTooOld = 4103
)

// --- 9xxx: Internal / Infra ---
const (
	// ErrInternalDomainMissing — domain missing in context (middleware issue).
	ErrInternalDomainMissing = 9001
	// ErrInternalBDNameMissing — bd_name missing in context (middleware issue).
	ErrInternalBDNameMissing = 9002
	// ErrInternalServiceConfigMissing — list_services config missing for domain.
	ErrInternalServiceConfigMissing = 9003
	// ErrInternalDB — generic DB/internal error.
	ErrInternalDB = 9004
	// ErrInternalUnexpected — generic unexpected server error.
	ErrInternalUnexpected = 9005
	// ErrInternalTelegramProviderConfigMissing — telegram provider config missing for domain.
	ErrInternalTelegramProviderConfigMissing = 9006
	// ErrInternalTelegramBotTokenMissing — telegram client_secret empty.
	ErrInternalTelegramBotTokenMissing = 9007
	// ErrInternalRoleConfigMissing — roles config missing/misconfigured.
	ErrInternalRoleConfigMissing = 9008
)
