# Error Handling

## When to Use

Every function that can fail — which is nearly every function in Go. This pattern replaces try/except from Python.

## Pattern

```go
func DoSomething(input string) (*Result, error) {
    // 1. Validate input — early return with descriptive error.
    if input == "" {
        return nil, fmt.Errorf("input is required")
    }

    // 2. Call a function that can fail — wrap the error with context.
    data, err := fetchData(input)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch data for %q: %w", input, err)
    }

    // 3. Another fallible call — same pattern.
    result, err := process(data)
    if err != nil {
        return nil, fmt.Errorf("failed to process data: %w", err)
    }

    return result, nil
}
```

## Key Rules

- Always wrap with `%w` (not `%v`) so callers can use `errors.Is()` / `errors.As()` to inspect the root cause.
- Add context to the message: describe *what operation failed*, not just "error occurred". Good: `"failed to load CSV: %w"`. Bad: `"error: %w"`.
- Return `nil` for the value when returning an error. Return `nil` for the error when returning a value.
- Check `err != nil` immediately after every call that returns an error — never defer the check.
- Multiple early returns are idiomatic Go. Each guard clause handles one failure mode and returns. The "happy path" flows down the left edge of the function.
- In HTTP handlers, map errors to status codes: validation errors -> 400, internal failures -> 500, upstream failures -> 502.

## Example from This Codebase

`config/config.go` — `Load()` has 7 early return points: 3 for missing required env vars, 2 for invalid integers (`strconv.Atoi`), 1 for invalid boolean (`strconv.ParseBool`), and 1 for cross-field validation (TestMode true but TestEmails empty). Each returns `nil, fmt.Errorf("descriptive message: %w", err)`.

`mailer/sender.go` — `SendBatch` has 3 error returns: BuildMail failure (template error), client.Send failure (network), and status >= 400 (API rejection). Each wraps the original error with `%w`.
