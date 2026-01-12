# Roadmap: NomadCrew Backend

## Completed Milestones

- [v1.0 Codebase Refactoring](milestones/v1.0-ROADMAP.md) (Phases 1-12) — SHIPPED 2026-01-12
- [v1.1 Infrastructure Migration](milestones/v1.1-ROADMAP.md) (Phases 13-19) — SHIPPED 2026-01-12

---

## Milestone: v1.2 — Developer Experience & Mobile Prep

### Overview

Optimize development workflow for mobile app development and prepare for frontend integration work.

---

## Phase 20: Windows DevX for Mobile Development
**Status:** Not Started
**Research Required:** Yes (WSL2 performance tuning, Android toolchain optimization)

**Goal:** Streamline Windows development experience for building Android/iOS mobile apps with the NomadCrew frontend

**Background:**
Current machine setup analysis (2026-01-12):
- Windows 11 with Git Bash (MSYS2), no shell configuration
- NVM with Node 23.2.0 active, Expo 6.3.10, EAS 16.15.0
- Android SDK installed with ANDROID_HOME set correctly
- WSL2 Ubuntu available but workflow not configured
- Docker Desktop with WSL2 integration working
- Multiple dev tools installed but not optimized for mobile dev workflow

**Scope:**
- Shell configuration (Git Bash .bashrc)
- Android development workflow optimization
- WSL2 integration for React Native (optional path)
- Environment variable cleanup
- Development scripts and aliases
- VS Code workspace configuration

**Key Tasks:**

### 1. Shell Configuration
- Create `.bashrc` with NomadCrew-specific aliases
- Add mobile dev shortcuts (start emulator, adb commands, expo scripts)
- Clean up PATH duplicates and fix `%M2_HOME%` reference
- Configure prompt with git branch display

### 2. Android Toolchain Optimization
- Verify Android SDK components are up-to-date
- Configure Gradle for faster builds (daemon, parallel execution)
- Set up Android emulator quick-launch scripts
- Configure `local.properties` for frontend project
- Test `expo run:android` workflow end-to-end

### 3. Development Workflow Scripts
- Create `dev-start.sh` - one-command to start backend + frontend
- Create `android-dev.sh` - launch emulator + connect + start metro
- Create `clean-all.sh` - nuclear option for when things break
- Document common troubleshooting commands

### 4. VS Code Workspace Setup
- Create multi-root workspace for backend + frontend
- Configure recommended extensions
- Set up launch configurations for debugging
- Add task definitions for common operations

### 5. Environment Documentation
- Document current setup in `.planning/DEVX.md`
- Create onboarding guide for new developers
- List required tools and versions
- Troubleshooting FAQ

**Research Topics:**
- WSL2 vs native Windows for React Native performance
- Gradle build cache optimization
- Metro bundler performance tuning on Windows
- Android emulator hardware acceleration settings

**Success Criteria:**
- `npm run android` works reliably from Git Bash
- Emulator starts in < 30 seconds
- Hot reload works consistently
- Single command to start full development environment
- New developer can set up in < 1 hour with guide

**Dependencies:** None (can run in parallel with other phases)

---

## Progress Summary

| Milestone | Phases | Status | Shipped |
|-----------|--------|--------|---------|
| v1.0 Codebase Refactoring | 1-12 | Complete | 2026-01-12 |
| v1.1 Infrastructure Migration | 13-19 | Complete | 2026-01-12 |
| v1.2 Developer Experience | 20+ | Not Started | - |

**Total Phases Completed:** 19
**Current Production:** https://api.nomadcrew.uk (AWS EC2 + Coolify)

---

*Created: 2026-01-10*
*Last Updated: 2026-01-12 after v1.1 milestone*
