// Package cfgsave is the transactional write boundary for RunWisp
// configuration. It validates changes before writing, serializes writers,
// journals durable state, preserves file metadata, and recovers interrupted
// multi-file saves at startup.
package cfgsave
