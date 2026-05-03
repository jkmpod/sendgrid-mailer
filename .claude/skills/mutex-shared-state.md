# Mutex-Protected Shared State

## When to Use

Any package-level variable or struct field that is read/written by multiple goroutines. HTTP handlers run in separate goroutines, so any shared state they access needs protection.

## Pattern

```go
package handlers

import "sync"

// Declare the mutex next to the variables it protects.
var (
    mu      sync.Mutex
    counter int
    items   []Item
)

// Setter — lock, modify, unlock.
func IncrementCounter() {
    mu.Lock()
    defer mu.Unlock()
    counter++
}

// Getter — lock, copy, unlock, return the copy.
func GetItems() []Item {
    mu.Lock()
    defer mu.Unlock()
    copied := make([]Item, len(items))
    copy(copied, items)
    return copied
}

// Bounded append — cap the slice to prevent unbounded growth.
func AppendItem(item Item) {
    mu.Lock()
    defer mu.Unlock()
    items = append(items, item)
    if len(items) > 50 {
        items = items[len(items)-50:]
    }
}
```

### When to use RWMutex instead

```go
var mu sync.RWMutex

// Multiple readers can hold RLock simultaneously.
func GetValue() string {
    mu.RLock()
    defer mu.RUnlock()
    return value
}

// Writer needs exclusive Lock.
func SetValue(v string) {
    mu.Lock()
    defer mu.Unlock()
    value = v
}
```

Use `RWMutex` when reads are much more frequent than writes. Use plain `Mutex` when reads and writes are balanced or infrequent.

## Key Rules

- Always `defer mu.Unlock()` to prevent forgetting — even if the function is short.
- **Return copies of slices/maps**, not references to internal data. Without `copy()`, the caller holds a reference to the same backing array and can corrupt internal state.
- Group the mutex declaration next to the variables it protects. Add a comment naming what it guards.
- One mutex can guard multiple related variables (e.g., `lastSubject` and `sendLog` share `subjectMu`).
- Test with concurrent goroutines using `sync.WaitGroup` and `t.Parallel()` to trigger the race detector.

## Example from This Codebase

`server/handlers/state.go` — `subjectMu sync.Mutex` guards both `lastSubject` and `sendLog`. `GetSendLog()` returns a defensive copy via `make` + `copy`. `AppendSendLog` caps the slice at 50 entries.

`server/handlers/compose.go` — `mu sync.RWMutex` guards `lastColumns` and `lastFilePath`. Uses `RLock` for reads in `HandleCompose` and `Lock` for writes in `SetLastColumns`/`SetLastFilePath`.
