package api

// Package api contains REST handlers consumed by the Next.js dashboard.
type Handler struct{}

func New() *Handler { return &Handler{} }
