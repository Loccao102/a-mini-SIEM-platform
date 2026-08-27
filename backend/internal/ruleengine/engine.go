package ruleengine

// Package ruleengine evaluates enabled rules against normalized Elasticsearch events.
type Engine struct{}

func New() *Engine { return &Engine{} }
