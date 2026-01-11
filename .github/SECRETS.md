# GitHub Repository Secrets

This document lists the secrets required for GitHub Actions workflows.

## Current Secrets (Coolify + GitHub App)

**No GitHub secrets required for deployment!**

Coolify uses GitHub App integration which handles repository access and deployment triggers automatically. No webhook URLs or secrets needed in GitHub.

## Deprecated Secrets (Cloud Run)

These secrets were used for Cloud Run deployment and can be removed after migration is verified stable:

| Secret Name | Purpose | Status |
|-------------|---------|--------|
| `GCP_SA_KEY` | Google Cloud service account key | **Deprecated** - Remove after Phase 19 |

## Application Environment Variables

Environment variables for the application are configured in **Coolify**, not GitHub:

- Database connection (Neon PostgreSQL)
- Redis connection (Upstash)
- Supabase credentials
- JWT secrets
- API keys (Resend, Geoapify, Pexels)

**Coolify Dashboard:** http://3.130.209.141:8000

---

## Deployment Flow

1. Push to `main` branch
2. **GitHub Actions** runs tests + security scan (parallel)
3. **Coolify GitHub App** auto-deploys (parallel)
4. Both complete independently

---

## Migration Checklist

- [x] GitHub App integration configured (no secrets needed)
- [ ] Remove `GCP_SA_KEY` after Phase 19 (Cloud Run decommissioning)

---

*Last updated: 2026-01-12*
*Phase: 15-cicd-pipeline-migration*
