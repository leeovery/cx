package state

// SandboxRegistryEnv names the env var holding the path of a file that lists
// test-owned state directories, one per line — the daemon-pgrep sandbox's
// cross-process ownership registry, extending the in-process filtering to
// sweeps run inside a test-spawned `portal` binary. A file rather than an
// inline list because dirs are appended after subprocess env slices are built.
// Nothing in a production build reads it.
const SandboxRegistryEnv = "PORTAL_TEST_SANDBOX_REGISTRY"
