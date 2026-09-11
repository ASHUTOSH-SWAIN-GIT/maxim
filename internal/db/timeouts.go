package db

import "time"

const (
	ConnectionTimeout    = 10 * time.Second
	MetadataTimeout      = 10 * time.Second
	BrowseTimeout        = 10 * time.Second
	QueryTimeout         = 30 * time.Second
	DefaultCellByteLimit = 64 * 1024
	DefaultPageByteLimit = 4 * 1024 * 1024
	MaximumPageSize      = 500
	MaximumBrowseTimeout = 2 * time.Minute
)
