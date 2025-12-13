package version

// Version is the application version, set via ldflags during build.
var Version = "devel"

// Commit is the git commit hash, set via ldflags during build.
var Commit = "0000000"

// CommitDate is the git commit date-time, set via ldflags during build.
var CommitDate = "unknown"
