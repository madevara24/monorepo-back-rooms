// Package contracts declares the JSON wire types shared between
// control-panel-service and node-agent-service. Every body that crosses the
// API/agent boundary is declared here once, so a one-sided field rename
// fails to compile instead of silently decoding as a zero value.
package contracts
