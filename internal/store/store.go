package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	StatusPending    = "pending"
	StatusUploaded   = "uploaded"
	StatusSubmitted  = "submitted"
	StatusCompleted  = "completed"
	StatusDownloaded = "downloaded"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"
)

type Store struct {
	db *sql.DB
}

func Open(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateBatch(provider, inputFile string, requestCount int, endpoint string) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO batches (provider, input_file, request_count, endpoint, status) VALUES (?, ?, ?, ?, ?)`,
		provider, inputFile, requestCount, endpoint, StatusPending,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) MarkUploaded(id int64, remoteFileID string) error {
	_, err := s.db.Exec(
		`UPDATE batches SET status=?, remote_file_id=?, uploaded_at=? WHERE id=?`,
		StatusUploaded, remoteFileID, time.Now().UTC(), id,
	)
	return err
}

func (s *Store) MarkSubmitted(id int64, remoteBatchID string) error {
	_, err := s.db.Exec(
		`UPDATE batches SET status=?, remote_batch_id=?, submitted_at=? WHERE id=?`,
		StatusSubmitted, remoteBatchID, time.Now().UTC(), id,
	)
	return err
}

func (s *Store) MarkCompleted(id int64, succeeded, failed int) error {
	_, err := s.db.Exec(
		`UPDATE batches SET status=?, succeeded_count=?, failed_count=?, completed_at=? WHERE id=?`,
		StatusCompleted, succeeded, failed, time.Now().UTC(), id,
	)
	return err
}

func (s *Store) MarkDownloaded(id int64, outputFile string, succeeded, failed int) error {
	_, err := s.db.Exec(
		`UPDATE batches SET status=?, output_file=?, succeeded_count=?, failed_count=?, downloaded_at=? WHERE id=?`,
		StatusDownloaded, outputFile, succeeded, failed, time.Now().UTC(), id,
	)
	return err
}

func (s *Store) MarkFailed(id int64, errMsg string) error {
	_, err := s.db.Exec(
		`UPDATE batches SET status=?, error=?, completed_at=? WHERE id=?`,
		StatusFailed, errMsg, time.Now().UTC(), id,
	)
	return err
}

func (s *Store) MarkCancelled(id int64) error {
	_, err := s.db.Exec(
		`UPDATE batches SET status=?, completed_at=? WHERE id=?`,
		StatusCancelled, time.Now().UTC(), id,
	)
	return err
}

func (s *Store) UpdatePollStatus(id int64, remoteStatus string) error {
	_, err := s.db.Exec(
		`UPDATE batches SET remote_status=?, last_polled_at=? WHERE id=?`,
		remoteStatus, time.Now().UTC(), id,
	)
	return err
}

const batchColumns = `id, provider, input_file, request_count, remote_file_id, remote_batch_id, endpoint, status, error, output_file, remote_status, remote_json, succeeded_count, failed_count, created_at, uploaded_at, submitted_at, completed_at, downloaded_at, last_polled_at`

func (s *Store) GetBatch(id int64) (*Batch, error) {
	return s.scanOne(`SELECT `+batchColumns+` FROM batches WHERE id=?`, id)
}

func (s *Store) GetBatchByRemoteID(remoteID string) (*Batch, error) {
	return s.scanOne(`SELECT `+batchColumns+` FROM batches WHERE remote_batch_id=?`, remoteID)
}

func (s *Store) ListBatches(providerFilter string) ([]*Batch, error) {
	query := `SELECT ` + batchColumns + ` FROM batches`
	var args []any
	if providerFilter != "" {
		query += ` WHERE provider=?`
		args = append(args, providerFilter)
	}
	query += ` ORDER BY created_at DESC`
	return s.scanMany(query, args...)
}

func (s *Store) ListNonTerminal(providerFilter string) ([]*Batch, error) {
	query := `SELECT ` + batchColumns + ` FROM batches WHERE status NOT IN (?, ?, ?)`
	args := []any{StatusDownloaded, StatusFailed, StatusCancelled}
	if providerFilter != "" {
		query += ` AND provider=?`
		args = append(args, providerFilter)
	}
	query += ` ORDER BY created_at DESC`
	return s.scanMany(query, args...)
}

func (s *Store) scanOne(query string, args ...any) (*Batch, error) {
	row := s.db.QueryRow(query, args...)
	b := &Batch{}
	err := row.Scan(
		&b.ID, &b.Provider, &b.InputFile, &b.RequestCount,
		&b.RemoteFileID, &b.RemoteBatchID, &b.Endpoint,
		&b.Status, &b.Error, &b.OutputFile,
		&b.RemoteStatus, &b.RemoteJSON,
		&b.SucceededCount, &b.FailedCount,
		&b.CreatedAt, &b.UploadedAt, &b.SubmittedAt,
		&b.CompletedAt, &b.DownloadedAt, &b.LastPolledAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return b, err
}

func (s *Store) scanMany(query string, args ...any) ([]*Batch, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batches []*Batch
	for rows.Next() {
		b := &Batch{}
		if err := rows.Scan(
			&b.ID, &b.Provider, &b.InputFile, &b.RequestCount,
			&b.RemoteFileID, &b.RemoteBatchID, &b.Endpoint,
			&b.Status, &b.Error, &b.OutputFile,
			&b.RemoteStatus, &b.RemoteJSON,
			&b.SucceededCount, &b.FailedCount,
			&b.CreatedAt, &b.UploadedAt, &b.SubmittedAt,
			&b.CompletedAt, &b.DownloadedAt, &b.LastPolledAt,
		); err != nil {
			return nil, err
		}
		batches = append(batches, b)
	}
	return batches, rows.Err()
}
