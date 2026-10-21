package repository

import (
	"context"
	"database/sql"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
)

const releaseColumns = `id, version, commit_sha, checksum, signature, artifact_url, created_at`

func scanRelease(row rowScanner) (entities.Release, error) {
	var r entities.Release
	err := row.Scan(&r.ID, &r.Version, &r.CommitSHA, &r.Checksum, &r.Signature, &r.ArtifactURL, &r.CreatedAt)
	return r, err
}

// LatestReleaseVersion returns the highest published release version, or
// 0 if no release has been published yet. The publish handler uses this
// to enforce that every new release's version is strictly greater, so
// the catalog can never go backwards.
func LatestReleaseVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT max(version) FROM releases`).Scan(&version)
	if err != nil {
		return 0, err
	}
	return int(version.Int64), nil
}

// InsertRelease records a release the publish handler already verified
// the signature of.
func InsertRelease(ctx context.Context, db *sql.DB, r entities.Release) (entities.Release, error) {
	row := db.QueryRowContext(ctx, `
		INSERT INTO releases (version, commit_sha, checksum, signature, artifact_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+releaseColumns+`
	`, r.Version, r.CommitSHA, r.Checksum, r.Signature, r.ArtifactURL)
	return scanRelease(row)
}

// ListReleases returns every published release, newest version first.
func ListReleases(ctx context.Context, db *sql.DB) ([]entities.Release, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT `+releaseColumns+`
		FROM releases
		ORDER BY version DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []entities.Release
	for rows.Next() {
		r, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
