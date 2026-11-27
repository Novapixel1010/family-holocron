// Package plugins is the compile-time plugin registry.
//
// Every optional feature lives in its own folder, plugins/<name>/, containing its
// Go code (routes, migrations, hooks) and its web UI (plugins/<name>/web). To add a
// plugin, list its Go constructor in All below and import its web entry in
// plugins/web.ts. Users can then turn it on or off in Settings → Plugins.
package plugins

import (
	"github.com/novapixel1010/family-holocron/plugin"
	"github.com/novapixel1010/family-holocron/plugins/stats"
	"github.com/novapixel1010/family-holocron/plugins/task"
)

// All returns every compiled-in plugin.
func All() []plugin.Plugin {
	return []plugin.Plugin{
		stats.New(),
		task.New(),
	}
}
