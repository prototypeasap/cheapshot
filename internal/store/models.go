package store

import (
	"database/sql"
	"time"
)

type Batch struct {
	ID             int64
	Provider       string
	InputFile      string
	RequestCount   int
	RemoteFileID   sql.NullString
	RemoteBatchID  sql.NullString
	Endpoint       sql.NullString
	Status         string
	Error          sql.NullString
	OutputFile     sql.NullString
	RemoteStatus   sql.NullString
	RemoteJSON     sql.NullString
	SucceededCount int
	FailedCount    int
	CreatedAt      time.Time
	UploadedAt     sql.NullTime
	SubmittedAt    sql.NullTime
	CompletedAt    sql.NullTime
	DownloadedAt   sql.NullTime
	LastPolledAt   sql.NullTime
}
