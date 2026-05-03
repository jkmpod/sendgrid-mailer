# Table-Driven Tests

## When to Use

Testing any function with multiple input/output scenarios. This is the standard Go testing pattern — use it by default.

## Pattern

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr string // substring expected in error; "" means no error
    }{
        {
            name:  "valid input",
            input: "hello",
            want:  "HELLO",
        },
        {
            name:    "empty input returns error",
            input:   "",
            wantErr: "input is required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Foo(tt.input)

            if tt.wantErr != "" {
                if err == nil {
                    t.Fatalf("expected error containing %q, got nil", tt.wantErr)
                }
                if !strings.Contains(err.Error(), tt.wantErr) {
                    t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if got != tt.want {
                t.Errorf("Foo(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

## Key Rules

- Use `tt` as the loop variable name (consistent across this codebase).
- Use `t.Run(tt.name, ...)` so failures show which case failed.
- Use `t.Fatalf` for "cannot continue" failures; `t.Errorf` for "wrong value but keep going".
- Use `t.Helper()` in any helper function so error locations point to the caller.
- Put the `wantErr` check first with an early `return` — keeps the success path unindented.
- For environment-dependent tests, write a `clearEnv(t *testing.T)` helper using `os.LookupEnv` + `t.Cleanup` to save/restore state.

## Example from This Codebase

`config/config_test.go` — `TestLoad` uses this pattern with 11 test cases covering valid env vars, missing required vars, invalid integers, test mode with/without emails, and invalid booleans. Each case sets env vars via a `map[string]string` and checks both the returned `Config` fields and the error message substring.

`server/handlers/send_test.go` — `TestHandleSend` uses 5 validation cases (missing filePath, subject, template, invalid JSON, CSV not found) all in one table with `wantStatus` and `wantErr` fields.
