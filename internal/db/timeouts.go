package db

import "time"

const (
	ConnectionTimeout = 10 * time.Second
	MetadataTimeout   = 10 * time.Second
	BrowseTimeout     = 10 * time.Second
	QueryTimeout      = 30 * time.Second
)
