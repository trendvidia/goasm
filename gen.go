package main

//go:generate go run ./cmd/goasm -arch arm64 -in templates/arm64.goasm -out jit/emit_arm64.go
//go:generate go run ./cmd/goasm -arch amd64 -in templates/amd64.goasm -out jit/emit_amd64.go
