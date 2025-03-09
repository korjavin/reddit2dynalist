# CLAUDE.md - Guide for Reddit2Dynalist Project

## Build/Run Commands
- Build: `go build`
- Run locally: `./reddit2dynalist`
- Run tests: `go test ./...`
- Run single test: `go test -run TestName` 
- Build Docker: `docker build -t reddit2dynalist .`
- Lint: `golangci-lint run`
- Format code: `gofmt -w .`

## Code Style Guidelines
- **Imports**: Group standard library imports first, then third-party, then local packages
- **Naming**: Use CamelCase for public names, camelCase for private names
- **Error Handling**: Always check errors, use descriptive messages with `log.Printf/Fatal` 
- **Types**: Prefer strong typing with custom types/structs over primitive types
- **Documentation**: Add comments for public functions and complex logic
- **Testing**: Write unit tests for business logic and integration tests for API calls
- **Concurrency**: Use contexts for cancellation signals and timeouts

## Project Structure
- Main application logic in `main.go`
- Follow Go project layout conventions for larger features
- Use environment variables for configuration