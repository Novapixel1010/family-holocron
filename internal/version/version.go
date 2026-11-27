// Package version holds build-time version information.
package version

// Version is overridden at build time with -ldflags "-X .../internal/version.Version=1.2.3".
var Version = "0.1.0-dev"

// BackupFormat is the version of the backup archive layout produced by this build.
const BackupFormat = 1
