package store

import (
	"context"
	"database/sql"
)

const nodeColumns = `id, name, address, token_hash, capacity, version, last_heartbeat, created_at`

func scanNode(row rowScanner) (Node, error) {
	var n Node
	err := row.Scan(&n.ID, &n.Name, &n.Address, &n.TokenHash, &n.Capacity, &n.Version, &n.LastHeartbeat, &n.CreatedAt)
	return n, err
}

// NodeByTokenHash looks up a node by the SHA-256 hash of its bearer
// token. It returns sql.ErrNoRows if none matches.
func NodeByTokenHash(ctx context.Context, db *sql.DB, tokenHash string) (Node, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+nodeColumns+`
		FROM nodes
		WHERE token_hash = $1
	`, tokenHash)
	return scanNode(row)
}

// AnyAvailableNode returns the node that heartbeated most recently, so a
// new database lands on a node the control plane has actually heard from.
// It returns sql.ErrNoRows if no node has ever heartbeated.
func AnyAvailableNode(ctx context.Context, db *sql.DB) (Node, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+nodeColumns+`
		FROM nodes
		WHERE last_heartbeat IS NOT NULL
		ORDER BY last_heartbeat DESC
		LIMIT 1
	`)
	return scanNode(row)
}

// UpdateNodeHeartbeat records a node's latest capacity report and the time
// it checked in. The agent's heartbeat body carries no version field (see
// internal/node/heartbeat.go), so version is left untouched here.
func UpdateNodeHeartbeat(ctx context.Context, db *sql.DB, nodeID string, capacity []byte) error {
	_, err := db.ExecContext(ctx, `
		UPDATE nodes SET capacity = $2, last_heartbeat = now()
		WHERE id = $1
	`, nodeID, capacity)
	return err
}

// UpsertNodeContainers replaces a node's reported containers with exactly
// the set it just sent in a heartbeat: existing rows are updated, new ones
// inserted, and rows for containers the node no longer reports are
// deleted.
func UpsertNodeContainers(ctx context.Context, db *sql.DB, nodeID string, containers []NodeContainer) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	reported := make([]string, len(containers))
	for i, c := range containers {
		reported[i] = c.DatabaseID
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO node_containers (node_id, database_id, status, port, updated_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (node_id, database_id)
			DO UPDATE SET status = EXCLUDED.status, port = EXCLUDED.port, updated_at = now()
		`, nodeID, c.DatabaseID, c.Status, c.Port); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM node_containers
		WHERE node_id = $1 AND NOT (database_id = ANY($2::uuid[]))
	`, nodeID, reported); err != nil {
		return err
	}

	return tx.Commit()
}
