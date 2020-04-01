package helpers

import (
	"github.com/andersfylling/disgord"
	"strings"
)

func GetPermissionsText(permissions disgord.PermissionBit) string {
	if permissions == 0 {
		return "/"
	}
	var result string
	if permissions&disgord.PermissionAdministrator == disgord.PermissionAdministrator {
		result += "Administrator, "
	}
	if permissions&disgord.PermissionViewAuditLogs == disgord.PermissionViewAuditLogs {
		result += "View Audit Log, "
	}
	if permissions&disgord.PermissionManageServer == disgord.PermissionManageServer {
		result += "Manage Server, "
	}
	if permissions&disgord.PermissionManageRoles == disgord.PermissionManageRoles {
		result += "Manage Roles, "
	}
	if permissions&disgord.PermissionManageChannels == disgord.PermissionManageChannels {
		result += "Manage Channels, "
	}
	if permissions&disgord.PermissionKickMembers == disgord.PermissionKickMembers {
		result += "Kick Members, "
	}
	if permissions&disgord.PermissionBanMembers == disgord.PermissionBanMembers {
		result += "Ban Members, "
	}
	if permissions&disgord.PermissionCreateInstantInvite == disgord.PermissionCreateInstantInvite {
		result += "Create Instant Invite, "
	}
	if permissions&disgord.PermissionChangeNickname == disgord.PermissionChangeNickname {
		result += "Change Nickname, "
	}
	if permissions&disgord.PermissionManageNicknames == disgord.PermissionManageNicknames {
		result += "Manage Nicknames, "
	}
	if permissions&disgord.PermissionManageEmojis == disgord.PermissionManageEmojis {
		result += "Manage Emojis, "
	}
	if permissions&disgord.PermissionManageWebhooks == disgord.PermissionManageWebhooks {
		result += "Manage Webhooks, "
	}
	if permissions&disgord.PermissionReadMessages == disgord.PermissionReadMessages {
		result += "View Channels, "
	}
	if permissions&disgord.PermissionSendMessages == disgord.PermissionSendMessages {
		result += "Send Messages, "
	}
	if permissions&disgord.PermissionSendTTSMessages == disgord.PermissionSendTTSMessages {
		result += "Send TTS Messages, "
	}
	if permissions&disgord.PermissionManageMessages == disgord.PermissionManageMessages {
		result += "Manage Messages, "
	}
	if permissions&disgord.PermissionEmbedLinks == disgord.PermissionEmbedLinks {
		result += "Embed Links, "
	}
	if permissions&disgord.PermissionAttachFiles == disgord.PermissionAttachFiles {
		result += "Attach Files, "
	}
	if permissions&disgord.PermissionReadMessageHistory == disgord.PermissionReadMessageHistory {
		result += "Read Message History, "
	}
	if permissions&disgord.PermissionMentionEveryone == disgord.PermissionMentionEveryone {
		result += "Mention Everyone, "
	}
	if permissions&disgord.PermissionUseExternalEmojis == disgord.PermissionUseExternalEmojis {
		result += "Use External Emojis, "
	}
	if permissions&disgord.PermissionAddReactions == disgord.PermissionAddReactions {
		result += "Add Reactions, "
	}
	if permissions&disgord.PermissionVoiceConnect == disgord.PermissionVoiceConnect {
		result += "Voice Connect, "
	}
	if permissions&disgord.PermissionVoiceSpeak == disgord.PermissionVoiceSpeak {
		result += "Voice Speak, "
	}
	if permissions&disgord.PermissionVoiceMuteMembers == disgord.PermissionVoiceMuteMembers {
		result += "Voice Mute Members, "
	}
	if permissions&disgord.PermissionVoiceDeafenMembers == disgord.PermissionVoiceDeafenMembers {
		result += "Voice Deafen Members, "
	}
	if permissions&disgord.PermissionVoiceMoveMembers == disgord.PermissionVoiceMoveMembers {
		result += "Voice Move Members, "
	}
	if permissions&disgord.PermissionVoiceUseVAD == disgord.PermissionVoiceUseVAD {
		result += "Voice Use Voice Acivity, "
	}
	result = strings.TrimRight(result, ", ")
	return result
}
