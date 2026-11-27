// Web half of the compile-time plugin registry (see plugins.go). Each import
// registers that plugin's UI slots; a slot is only shown while the server reports
// the plugin as enabled.
import "./stats/web";
import "./task/web";
