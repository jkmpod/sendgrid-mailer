# Session Tracking (Auto-Update)

## When to Use

At the **end of every Claude Code session** on this project, before the conversation ends. This is an automatic behavior — do not wait to be asked.

## Trigger Conditions

Update the tracker when ANY of these are true:
- The user says they're done, wrapping up, or ending the session
- The user asks to commit final changes
- The conversation is about to end naturally (all tasks complete)
- The user explicitly asks to update the tracker

## Pattern

### Step 1: Determine session number

```
Read the last row of the Session Log table in session-tracker.md.
New session number = last session number + 1.
```

### Step 2: Gather data

| Field | How to Get It |
|-------|--------------|
| Date | Today's date from the system (YYYY-MM-DD) |
| Duration | Estimate from conversation start to now (round to nearest 5 min) |
| Focus | One-line summary of what was built/changed this session |
| Model | From the environment info (e.g., `claude-opus-4-6`) |
| Effort | From the environment info (e.g., `default`) |
| Messages | Count of assistant responses in this conversation (estimate) |
| Output Tokens | Estimate: ~messages × 300 for code-heavy sessions, ~messages × 150 for discussion-heavy. Mark with `~` prefix to indicate estimate. |
| Cache Read | Not available mid-session. Write `TBD` — user fills in later from dashboard. |
| Total Tokens | Not available mid-session. Write `TBD`. |
| Notes | Key decisions, anomalies, or metacognitive observations |

### Step 3: Append row to session-tracker.md

```markdown
| 10 | 2026-03-28 | ~25 | Session tracking skill | ~15 | ~4,500 | TBD | TBD | Auto-tracked by skill |
```

### Step 4: Update running totals

Increment the "Total sessions" count. Add estimated output tokens to the running total. Leave cache/total tokens unchanged (user corrects after checking dashboard).

### Step 5: Append to learning.md (if applicable)

If the session involved Go code changes, learning decisions, or new patterns, also append a `## Session N` section to `learning.md` following the established format (exports, error points, idioms).

## Key Rules

- **Always auto-update** — this runs at session end without being asked. If you forget, the user loses data.
- **Estimates are fine** — prefix with `~` to signal approximation. The user corrects from their Claude Code dashboard later. Precision improves over time.
- **Do not commit** — only update the file. The user decides when to commit.
- **Do not update for trivial interactions** — if the session was just a quick question with no code or file changes, skip the tracker update.
- **One row per session** — if a conversation spans multiple logical sessions (e.g., user says "let's start Session 11 now"), create one row per logical session.

## Example from This Codebase

`session-tracker.md` — Contains 9 historical rows reconstructed from JSONL conversation logs in Session 10. Future rows are appended automatically by this skill.
