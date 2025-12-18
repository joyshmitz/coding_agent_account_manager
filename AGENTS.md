# Agent Coordination Board

## Active Agents
- **Gemini**: Fixed critical bug in rotation last-activation query (`caam-j06` regression fix).
- **Codex (GPT-5.2)**: Closed `caam-3nx`, implemented `caam-5ed`, and finished smart profile rotation (`caam-ewh`). Closed `caam-j06`, `caam-d8x`, and `caam-l4q`.
- **LilacCastle (Claude Opus 4.5)**: Bug fixes in rotation/activate code; closed stealth epic `caam-e8o`.

## Project Status
✅ **All 102 beads closed** - Project feature complete!

## Task Queue
- [x] Investigate codebase
- [x] `caam-6gi`: Implement penalty decay logic
- [x] Fix project glob associations (internal/project/store.go)
- [x] Fix profile atomic saves (internal/profile/profile.go)
- [x] `caam-e36`: Proactive Token Refresh (committed)
- [x] Fix `getProfileHealth` wiring in `root.go`
- [x] `caam-3nx`: Data Safety & Recovery (original backups, uninstall, protected profiles)
- [x] `caam-5ed`: Cooldown tracking (DB + CLI + activate integration)
- [x] `caam-ewh`: Smart profile rotation (--auto + tests/docs)
- [x] `caam-j06`: Fix rotation last-activation query
- [x] `caam-d8x`: Reliability: atomic config save + flush URL capture
- [x] `caam-l4q`: Health formatting: remove deprecated strings.Title

## Messages
(None)
