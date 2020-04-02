package gocto

import (
	"github.com/jonas747/discordgo"
)

type Permissions int

func PermissionsForRole(role *discordgo.Role) Permissions {
	return Permissions(role.Permissions)
}

func PermissionsForMember(guild *discordgo.Guild, member *discordgo.Member) Permissions {
	if member.User.ID == guild.OwnerID {
		return Permissions(discordgo.PermissionAll)
	}
	bits := 0
	// Combine all permissions from every role.
	for _, rID := range member.Roles {
		var role *discordgo.Role
		for _, gRole := range guild.Roles {
			if gRole.ID == rID {
				role = gRole
				break
			}
		}
		if role != nil {
			bits |= role.Permissions
		}
	}
	return Permissions(bits)
}

func (perms Permissions) Has(bits int) bool {
	return (int(perms) & bits) == bits
}
