# Validate

Run the full nanowave validation suite:

1. Run `make build` to compile the CLI
2. Run `make test` to execute all Go tests
3. Run `make skills-source-validate` to check skill compliance
4. Run `go vet ./...` for static analysis

Report results as a structured summary with PASS/FAIL for each step.
If any step fails, explain what went wrong and suggest fixes.
