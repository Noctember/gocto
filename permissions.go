package gocto

import (
	"github.com/Noctember/disgord"
)

type Permissions int

func PermissionsForRole(role *disgord.Role) Permissions {
	return Permissions(role.Permissions)
}

func PermissionsForMember(guild *disgord.Guild, member *disgord.Member) Permissions {
	if member.User.ID == guild.OwnerID {
		return Permissions(disgord.PermissionAll)
	}
	bits := uint64(0)
	for _, rID := range member.Roles {
		var role *disgord.Role
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
