package utils

import (
	"anubis/app/core"
	"fmt"
	"strings"
)

// SplitRole разбивает строку роли из workspace_members.role на базовую роль и вариант.
// Примеры:
//
//	"owner"       -> ("owner", "")
//	"manager:all" -> ("manager", "all")
//
// IMPORTANT: в токены и БД пишем исходную строку ("manager:all"), а для проверок прав
// обычно сравниваем только base ("owner"/"admin"/...).
func SplitRole(role string) (base string, variant string) {
	role = strings.TrimSpace(role)
	if role == "" {
		return "", ""
	}

	parts := strings.SplitN(role, ":", 2)
	base = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		variant = strings.TrimSpace(parts[1])
	}
	return base, variant
}

// ValidateRoleAgainstConfig проверяет, что role (включая variant) описана в конфиге домена.
// Это необходимо, потому что в БД role теперь свободная строка (без CHECK constraint).
func ValidateRoleAgainstConfig(roles map[string]core.RoleConfig, role string) error {
	base, variant := SplitRole(role)
	if base == "" {
		return fmt.Errorf("empty role")
	}

	rc, ok := roles[base]
	if !ok {
		return fmt.Errorf("unknown role: %s", base)
	}

	if rc.Variants != nil {
		if variant == "" {
			return fmt.Errorf("role need variants")
		} else {
			if _, ok := rc.Variants[variant]; !ok {
				return fmt.Errorf("unknown role variant: %s:%s", base, variant)
			}
		}
	} else {
		return nil
	}

	return nil
}
