package common

import "strings"

// IsSidebarModuleRootOnly reports whether a sidebar module is restricted to
// root administrators by the global sidebar configuration.
func IsSidebarModuleRootOnly(section, module string) bool {
	OptionMapRWMutex.RLock()
	raw := OptionMap["SidebarModulesAdmin"]
	OptionMapRWMutex.RUnlock()
	if strings.TrimSpace(raw) == "" {
		return false
	}

	var parsed map[string]map[string]any
	if err := Unmarshal([]byte(raw), &parsed); err != nil {
		return false
	}
	sectionConfig := parsed[section]
	if sectionConfig == nil {
		return false
	}
	value, ok := sectionConfig[module+"RootOnly"]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	case float64:
		return typed == 1
	default:
		return false
	}
}

func CanViewSidebarModule(role int, section, module string) bool {
	return !IsSidebarModuleRootOnly(section, module) || role >= RoleRootUser
}
