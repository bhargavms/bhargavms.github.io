# Design language hook

Runs automatically when UI files are edited in Cursor Agent or Tab.

## Triggers

| Event | When |
|-------|------|
| `postToolUse` | After Agent `Write`, `StrReplace`, `EditNotebook`, or `ApplyPatch` |
| `afterFileEdit` | After Agent file edits |
| `afterTabFileEdit` | After Tab inline completions |

## UI paths

- `assets/css/**`
- `assets/js/main.js`, `assets/js/widgets.js`
- `layouts/**`
- `static/**/*.css`, `static/**/*.html`

## Behavior

1. Injects a **DESIGN.md cross-check checklist** into the agent context.
2. Runs lightweight static lint (box-shadow on containers, `transition: all`, cobalt link as button bg).

## Manual test

```bash
echo '{"hook_event_name":"afterFileEdit","file_path":"'"$(pwd)"'/assets/css/main.css","edits":[]}' \
  | .cursor/hooks/design-check.sh | python3 -m json.tool
```

## Enable in Cursor

Project hooks load in **trusted workspaces**. Check **Cursor Settings → Hooks** or the Hooks output channel if the hook does not fire.

Reference: [DESIGN.md](../../DESIGN.md)
