# Fix Tests

Run `make test` and fix any failing tests:

1. Run `go test ./... -v` to identify failures
2. For each failing test, read the test file and the source file
3. Determine the root cause
4. Fix the source code (not the test) unless the test itself has a bug
5. Re-run tests to verify the fix
6. Repeat until all tests pass

After all tests pass, also run:
- `go vet ./...`
- `make skills-source-validate`
