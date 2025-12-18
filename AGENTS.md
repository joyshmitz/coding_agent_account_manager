### Using caam for instant AI coding tool account switching

caam (Coding Agent Account Manager) manages auth files for AI coding CLIs to enable sub-second account switching. When you hit usage limits on "all you can eat" subscriptions (GPT Pro, Claude Max, Gemini Ultra), switch accounts instantly without browser login flows.

**Core mechanism:** Each tool stores OAuth tokens in specific files. caam backs them up with labels and restores them on demand. No proxies, no env vars—just file copies.

---

### Quick Reference

```bash
# Backup current auth (after logging in normally)
caam backup claude jeff-gmail-1
caam backup codex work-openai
caam backup gemini personal

# Instant switch (< 1 second)
caam activate claude jeff-gmail-2
caam activate codex backup-account

# Check what's active
caam status

# List all saved profiles
caam ls

# Show auth file locations
caam paths
```

---

### Auth File Locations

| Tool | Auth Files | Login Command |
|------|-----------|---------------|
| Codex CLI | `~/.codex/auth.json` | `codex login` |
| Claude Code | `~/.claude.json`, `~/.config/claude-code/auth.json` | `/login` in Claude |
| Gemini CLI | `~/.gemini/settings.json` | Start `gemini`, select "Login with Google" |

The vault stores backups at: `~/.local/share/caam/vault/<tool>/<profile>/`

---

### Commands

#### Auth File Swapping (Primary)

| Command | Description |
|---------|-------------|
| `caam backup <tool> <name>` | Save current auth to vault |
| `caam activate <tool> <name>` | Restore auth (instant switch!) |
| `caam status [tool]` | Show which profile is active |
| `caam ls [tool]` | List all saved profiles |
| `caam delete <tool> <name>` | Remove a saved profile |
| `caam paths [tool]` | Show auth file locations |
| `caam clear <tool>` | Remove auth files (logout) |

#### Vault Transfer

| Command | Description |
|---------|-------------|
| `caam export <tool>/<profile>` | Export profile to tar.gz (stdout or -o file) |
| `caam export --all` | Export entire vault |
| `caam import <file>` | Import profiles from tar.gz |
| `caam import <file> --as <tool>/<name>` | Import with rename |

#### Profile Isolation (Advanced)

For running multiple sessions simultaneously with fully isolated environments:

| Command | Description |
|---------|-------------|
| `caam profile add <tool> <name>` | Create isolated profile directory |
| `caam profile ls [tool]` | List isolated profiles |
| `caam profile delete <tool> <name>` | Delete isolated profile |
| `caam profile status <tool> <name>` | Show isolated profile status |
| `caam login <tool> <profile>` | Run login flow for isolated profile |
| `caam exec <tool> <profile> [-- args]` | Run CLI with isolated profile |

---

### Headless Server Workflows

Working on a headless server (SSH, containers, CI/CD) without a browser? Here's how to set up and use caam.

#### Setup: Device Code Authentication (Codex)

Codex supports device code flow - no browser on the server needed:

```bash
# On headless server
codex login --device-auth

# Output:
# Follow these steps to sign in with ChatGPT using device code authorization:
# 1. Open this link in your browser and sign in to your account
#    https://auth.openai.com/codex/device
# 2. Enter this one-time code (expires in 15 minutes)
#    FXZV-2QEFS

# Complete auth on your phone or any browser, then:
caam backup codex account1

# Repeat for additional accounts
codex logout
codex login --device-auth
caam backup codex account2
```

#### Setup: Vault Transfer (All Providers)

For Claude and Gemini (or if you prefer not to use device code), transfer credentials from a machine with a browser:

```bash
# On machine WITH browser (laptop, workstation)
claude               # Login via /login
caam backup claude work-account
caam export claude/work-account -o claude-work.tar.gz

# Transfer to headless server
scp claude-work.tar.gz user@server:~

# On headless server
caam import claude-work.tar.gz
caam activate claude work-account    # Ready to use!
```

Export multiple profiles at once:
```bash
# Export all Codex profiles
caam export codex --all -o codex-profiles.tar.gz

# Export entire vault
caam export --all -o full-vault.tar.gz
```

#### The Killer Workflow: Hit Limit → Switch → Resume

When you hit a usage limit mid-session, Codex shows:
```
■ You've hit your usage limit.
To continue this session, run codex resume 019b2e3d-b524-7c22-91da-47de9068d09a
```

**Instant recovery:**
```bash
# Switch to backup account (< 1 second!)
caam activate codex account2

# Resume your session with the new account
codex resume 019b2e3d-b524-7c22-91da-47de9068d09a "proceed"
```

The session context is preserved - only the auth changes. You continue exactly where you left off!

#### Why Sessions Survive Account Switches

Sessions are stored in `~/.codex/sessions/`, separate from auth (`~/.codex/auth.json`). When you run `caam activate`, only the auth file changes. Your session history stays intact, so `codex resume` works with any account.

---

### Workflow for AI Agents

When working on projects that require AI coding tools:

1. **Check current status**: `caam status` to see which accounts are active
2. **Before long sessions**: Ensure you have backup profiles ready
3. **When hitting limits**: `caam activate <tool> <next-profile>` and continue

The switching is atomic and instant—no need to restart sessions or wait for browser flows.

---

### Advanced: Parallel Sessions

For running multiple accounts simultaneously:

```bash
# Create isolated profiles
caam profile add codex work
caam profile add codex personal

# Login to each (one-time)
caam login codex work
caam login codex personal

# Run with specific profile
caam exec codex work -- "implement feature X"
caam exec codex personal -- "review code"
```

Each profile has its own HOME/CODEX_HOME with passthrough symlinks to your real .ssh, .gitconfig, etc.

---

### ast-grep vs ripgrep (quick guidance)

**Use `ast-grep` when structure matters.** It parses code and matches AST nodes, ignoring comments/strings.

**Use `ripgrep` when text is enough.** Fastest way to grep literals/regex.

**Rule of thumb:**
- Need correctness or will **apply changes** → `ast-grep`
- Need raw speed or just **hunting text** → `rg`

---

### UBS Quick Reference

**Golden Rule:** `ubs <changed-files>` before every commit. Exit 0 = safe.

```bash
ubs file.go              # Specific file
ubs --only=go src/       # Language filter
```

---

You should try to follow all best practices laid out in the file GOLANG_BEST_PRACTICES.md

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
