package store

// Well-known system settings.
const (
	SettingInstanceName = "instance.name"
)

// PluginEnabledSetting is the setting key toggling a plugin.
func PluginEnabledSetting(id string) string { return "plugin." + id + ".enabled" }
