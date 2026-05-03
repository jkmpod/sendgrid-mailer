# Server-Sent Events (SSE) Streaming

## When to Use

Long-running operations (batch sends, file processing, deployments) where the client needs real-time progress updates without polling.

## Pattern

### Server Side (Go)

```go
func HandleLongTask(svc *Service) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Check if the ResponseWriter supports flushing.
        flusher, ok := w.(http.Flusher)
        if !ok {
            // Fallback: return a single JSON response.
            result := svc.DoAll()
            writeJSON(w, http.StatusOK, result)
            return
        }

        // 2. Set SSE headers.
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        // 3. Stream progress events.
        for i, item := range items {
            result := svc.Process(item)
            sseEvent(w, "progress", map[string]interface{}{
                "step":  i + 1,
                "total": len(items),
                "status": result.Status,
            })
            flusher.Flush() // Push to client immediately.
        }

        // 4. Send a final summary event.
        sseEvent(w, "done", map[string]interface{}{
            "totalProcessed": len(items),
        })
        flusher.Flush()
    }
}

// Helper: write one SSE event.
func sseEvent(w http.ResponseWriter, event string, data interface{}) {
    jsonData, _ := json.Marshal(data)
    fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData)
}
```

### Client Side (JavaScript)

```javascript
fetch("/long-task", { method: "POST", body: JSON.stringify(payload) })
  .then(function(resp) {
    var ct = resp.headers.get("Content-Type") || "";

    // JSON fallback (e.g., test mode or no flusher support).
    if (ct.includes("application/json")) {
      return resp.json().then(handleResult);
    }

    // SSE: read the stream.
    var reader = resp.body.getReader();
    var decoder = new TextDecoder();
    var buffer = "";

    function pump() {
      return reader.read().then(function(result) {
        if (result.done) return;
        buffer += decoder.decode(result.value, { stream: true });
        var parts = buffer.split("\n\n");
        buffer = parts.pop(); // Keep incomplete chunk.
        parts.forEach(parseSSE);
        return pump();
      });
    }
    return pump();
  });

function parseSSE(raw) {
  var event = "", data = "";
  raw.split("\n").forEach(function(line) {
    if (line.startsWith("event: ")) event = line.substring(7);
    else if (line.startsWith("data: ")) data = line.substring(6);
  });
  if (!event || !data) return;
  var parsed = JSON.parse(data);
  // Handle "progress" and "done" events...
}
```

## Key Rules

- Each SSE message ends with **two newlines** (`\n\n`). The `event:` line names the event; the `data:` line carries the JSON payload.
- Always `flusher.Flush()` after writing — without it, the data sits in the buffer and the client sees nothing.
- Provide a JSON fallback for clients that don't support streaming (check `w.(http.Flusher)`).
- Once SSE streaming starts, the status code is always 200. Report errors as events, not HTTP status codes.
- On the client, use `fetch` + `getReader()` (not `EventSource`) because SSE here is sent via POST, and `EventSource` only supports GET.
- The client must split on `\n\n` and handle partial chunks (keep a buffer).

## Example from This Codebase

`server/handlers/send.go` — `HandleSend` streams batch results as `"batch"` events (with batch number, status, sent/error count) and a final `"done"` event with totals. Falls back to JSON for test mode and non-flusher environments.

`templates/index.html` — The `parseSSE` function and `pump()` reader handle the stream client-side, updating a progress bar and log panel in real time.
