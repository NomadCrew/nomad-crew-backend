# GitHub Repository Secrets

This document lists the secrets required for GitHub Actions workflows.

## Required Secrets (Coolify Deployment)

| Secret Name | Purpose | Where to Obtain | Status |
|-------------|---------|-----------------|--------|
| `COOLIFY_WEBHOOK_URL` | Coolify deployment webhook endpoint | Coolify Dashboard → Application → Webhooks | **Required** |
| `COOLIFY_WEBHOOK_SECRET` | Bearer token for webhook authentication | Coolify Dashboard → Application → Webhooks | **Required** |

### Setting Up Coolify Secrets

1. Open Coolify Dashboard: http://3.130.209.141:8000
2. Navigate to `nomad-crew-backend` application
3. Go to **Webhooks** section
4. Copy the webhook URL and secret
5. Add to GitHub:
   - Go to Repository → Settings → Secrets and variables → Actions
   - Add `COOLIFY_WEBHOOK_URL` with the webhook endpoint
   - Add `COOLIFY_WEBHOOK_SECRET` with the bearer token

## Deprecated Secrets (Cloud Run)

These secrets were used for Cloud Run deployment and can be removed after migration is verified stable:

| Secret Name | Purpose | Status |
|-------------|---------|--------|
| `GCP_SA_KEY` | Google Cloud service account key | **Deprecated** - Remove after Phase 19 |

## Other Secrets (Not Changed)

Secrets used by other workflows or services that remain unchanged:

| Secret Name | Used By | Notes |
|-------------|---------|-------|
| Various env vars | Application config | Configured in Coolify, not GitHub |

---

## Migration Checklist

- [ ] Add `COOLIFY_WEBHOOK_URL` to GitHub secrets
- [ ] Add `COOLIFY_WEBHOOK_SECRET` to GitHub secrets
- [ ] Verify first deployment succeeds via webhook
- [ ] Remove `GCP_SA_KEY` after Phase 19 (Cloud Run decommissioning)

---

*Last updated: 2026-01-12*
*Phase: 15-cicd-pipeline-migration*
