# Project

A Go service.

# Architecture

Entry points live under ./cmd. Keep everything else under ./internal.

# Coding conventions

- Format with `gofmt`; check with `go vet ./...`.
- Run `go test ./...` before committing.
- Prefer the standard library; add dependencies only when necessary.

# Important notes

Write anything Claude should always know about this project here.
