Build: go build -o ./bin/mrp cmd/mrp/\*.go
Test: go test ./...
Format: go fmt ./...; find . -name "*.go" -exec golines -w {}

- Be sure to compile and fix any errors when you’re done making a series of code changes
- Prefer running single tests, and not the whole test suite, for performance
- Error Handling: Return errors explicitly; prefer wrapping with context
- Naming: Use Go conventions (CamelCase for exported, camelCase for unexported)
- When something doesn't work, debug and fix it - don't start over with a simple version
- When you build put results in the bin/ folder
- Format after making any change