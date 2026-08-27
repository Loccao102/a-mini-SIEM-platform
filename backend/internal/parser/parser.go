package parser

// Package parser normalizes raw Redis Stream entries into Elasticsearch events.
// Source-specific regex definitions will be added here as the project grows.
type Parser struct{}

func New() *Parser { return &Parser{} }
