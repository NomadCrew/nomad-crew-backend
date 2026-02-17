# Wallet Feature — Infrastructure & Engineering Tasks

Tracking document for DevOps (manual/Terraform) and future engineering work required to fully operationalize the wallet feature.

**Last updated:** 2026-02-17

---

## DevOps Tasks (Infrastructure — Manual/Terraform)

### P0 — Before First Real User Upload

#### 1. Configure Coolify Persistent Volume

The Dockerfile creates `/var/data/wallet-files` with correct ownership, but the volume must be mapped in Coolify so files survive container redeployments.

- [ ] In Coolify UI: **Service → Advanced → Volume Mount**
- [ ] Map host path (e.g., `/opt/nomadcrew/wallet-files`) to container path `/var/data/wallet-files`
- [ ] Verify mount:
  ```bash
  docker inspect <container> | jq '.[0].Mounts'
  ```
- [ ] End-to-end test: upload a file via API, trigger a redeploy via git push, confirm the file is still retrievable

#### 2. Enable EBS Volume Encryption

AWS does not allow encrypting an existing EBS volume in-place. The process is: snapshot → create encrypted copy → swap volumes.

- [ ] **Option A — Manual (AWS Console):**
  1. EC2 → Volumes → Select the root volume
  2. Actions → Create Snapshot
  3. Snapshots → Select snapshot → Actions → Copy Snapshot → check **Encrypt**
  4. Create Volume from encrypted snapshot in the same AZ
  5. Stop instance → Detach old volume → Attach encrypted volume as `/dev/xvda` → Start instance
  6. Verify instance boots correctly
  7. Delete old unencrypted volume + snapshot

- [ ] **Option B — Terraform (for future instances):**
  Add to `infrastructure/aws/main.tf`:
  ```hcl
  resource "aws_instance" "api" {
    # ... existing config ...

    root_block_device {
      volume_size = 30
      volume_type = "gp3"
      encrypted   = true
    }
  }
  ```

- [ ] **Enable default encryption for the region** (prevents future unencrypted volumes):
  ```bash
  aws ec2 enable-ebs-encryption-by-default --region <region>
  ```

#### 3. Verify Volume Mount After Deploy

Run after completing task 1 and after every infrastructure change.

- [ ] SSH into EC2:
  ```bash
  ssh -i <key> ubuntu@<ip>
  ```
- [ ] Inspect container mounts:
  ```bash
  docker inspect $(docker ps -q -f name=api) | jq '.[0].Mounts'
  ```
  Expected output should include a mount with `"Destination": "/var/data/wallet-files"`.
- [ ] Verify host directory:
  ```bash
  ls -la /opt/nomadcrew/wallet-files/
  ```

---

### P1 — Within 2 Weeks

#### 4. Add EBS Snapshot Policy (Daily Backups)

Use AWS Data Lifecycle Manager for automated daily snapshots with 7-day retention.

- [ ] Tag the EBS volume for DLM targeting:
  ```bash
  aws ec2 create-tags --resources <volume-id> --tags Key=Backup,Value=true
  ```
- [ ] Create DLM lifecycle policy via Terraform:
  ```hcl
  resource "aws_iam_role" "dlm_lifecycle_role" {
    name = "dlm-lifecycle-role"

    assume_role_policy = jsonencode({
      Version = "2012-10-17"
      Statement = [{
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = { Service = "dlm.amazonaws.com" }
      }]
    })
  }

  resource "aws_iam_role_policy_attachment" "dlm_lifecycle" {
    role       = aws_iam_role.dlm_lifecycle_role.name
    policy_arn = "arn:aws:iam::aws:policy/service-role/AWSDataLifecycleManagerServiceRole"
  }

  resource "aws_dlm_lifecycle_policy" "daily_snapshots" {
    description        = "Daily EBS snapshots - 7 day retention"
    execution_role_arn = aws_iam_role.dlm_lifecycle_role.arn
    state              = "ENABLED"

    policy_details {
      resource_types = ["VOLUME"]

      schedule {
        name = "daily"

        create_rule {
          interval      = 24
          interval_unit = "HOURS"
          times         = ["03:00"]
        }

        retain_rule {
          count = 7
        }

        tags_to_add = {
          SnapshotCreator = "DLM"
        }
      }

      target_tags = {
        Backup = "true"
      }
    }
  }
  ```
- [ ] Apply Terraform and verify first snapshot is created within 24 hours

#### 5. Add CloudWatch Disk Usage Alarm

- [ ] Install CloudWatch agent on EC2 (if not already present):
  ```bash
  sudo apt-get install -y amazon-cloudwatch-agent
  ```
- [ ] Configure agent to report disk metrics (add to agent config JSON):
  ```json
  {
    "metrics": {
      "metrics_collected": {
        "disk": {
          "measurement": ["used_percent"],
          "resources": ["/"],
          "ignore_file_system_types": ["sysfs", "devtmpfs"]
        }
      }
    }
  }
  ```
- [ ] Create SNS topic for alerts:
  ```bash
  aws sns create-topic --name nomadcrew-disk-alerts
  aws sns subscribe --topic-arn <arn> --protocol email --notification-endpoint <ops-email>
  ```
- [ ] Create CloudWatch alarm at 80% disk usage:
  ```bash
  aws cloudwatch put-metric-alarm \
    --alarm-name "DiskUsageHigh" \
    --metric-name "disk_used_percent" \
    --namespace "CWAgent" \
    --statistic Average \
    --period 300 \
    --threshold 80 \
    --comparison-operator GreaterThanThreshold \
    --evaluation-periods 2 \
    --alarm-actions <sns-topic-arn> \
    --dimensions Name=path,Value=/ Name=InstanceId,Value=<instance-id>
  ```

#### 6. Add WALLET_SIGNING_KEY to Production Environment

The wallet HMAC signing key should be separate from `JWT_SECRET_KEY`. The code falls back to `JWT_SECRET_KEY` if `WALLET_SIGNING_KEY` is not set, but separation is recommended for security isolation.

- [ ] Generate a 256-bit key:
  ```bash
  openssl rand -hex 32
  ```
- [ ] Add `WALLET_SIGNING_KEY` to Coolify environment variables for the API service
- [ ] Redeploy and verify wallet endpoints still function correctly

#### 7. Run Migrations 000012 + 000013 on Production DB

Follow existing migration SoP (GitHub Gist → curl → docker exec psql).

- [ ] Run migration `000012` — creates `wallet_documents` table, enums, and indexes:
  ```bash
  docker exec -i <postgres-container> psql -U <user> -d <db> < 000012_create_wallet_documents.up.sql
  ```
- [ ] Run migration `000013` — creates `wallet_audit_log` table and indexes:
  ```bash
  docker exec -i <postgres-container> psql -U <user> -d <db> < 000013_create_wallet_audit_log.up.sql
  ```
- [ ] Verify tables exist:
  ```sql
  SELECT table_name FROM information_schema.tables
  WHERE table_schema = 'public' AND table_name LIKE 'wallet_%';
  ```

---

### P2 — Before 1K Users

#### 8. Migrate to Cloudflare R2 Object Storage

Replace local filesystem storage with S3-compatible object storage. R2 has zero egress fees, estimated ~$2/month at 10K users.

- [ ] Create R2 bucket in Cloudflare dashboard
- [ ] Generate R2 API tokens (S3-compatible credentials)
- [ ] Implement `R2FileStorage` struct satisfying the `FileStorage` interface
  - Use AWS S3 SDK (R2 is S3-compatible)
  - `Save()` → `PutObject`
  - `Delete()` → `DeleteObject`
  - `GetPath()` → return presigned URL (time-limited)
- [ ] Add config variables:
  | Variable | Description |
  |---|---|
  | `WALLET_STORAGE_BACKEND` | `r2` (currently `local`) |
  | `R2_BUCKET_NAME` | Bucket name |
  | `R2_ACCOUNT_ID` | Cloudflare account ID |
  | `R2_ACCESS_KEY_ID` | S3-compatible access key |
  | `R2_SECRET_ACCESS_KEY` | S3-compatible secret key |
  | `R2_ENDPOINT` | `https://<account-id>.r2.cloudflarestorage.com` |
- [ ] Write one-time migration script to move existing files from local to R2
- [ ] Update `ServeFileHandler` to redirect to presigned URL instead of using `c.File()`
- [ ] Test: upload, download, delete, presigned URL expiry
- [ ] Deploy with `WALLET_STORAGE_BACKEND=r2`

#### 9. Increase EBS Volume Size

If staying on local storage temporarily, increase disk to provide headroom.

- [ ] **Terraform** — update `volume_size` in `root_block_device`:
  ```hcl
  root_block_device {
    volume_size = 50  # was 30
    volume_type = "gp3"
    encrypted   = true
  }
  ```
- [ ] **Or AWS Console:** EC2 → Volumes → Modify Volume → set to 50 GB
- [ ] Extend the filesystem after resize:
  ```bash
  sudo growpart /dev/xvda 1
  sudo resize2fs /dev/xvda1   # ext4
  # or: sudo xfs_growfs /      # xfs
  ```
- [ ] Verify: `df -h /`

#### 10. Add File Virus Scanning

Scan uploaded files before persisting them.

- [ ] **Option A — ClamAV sidecar container:**
  - Add ClamAV service to Docker Compose / Coolify
  - Scan file bytes via `clamdscan` or TCP socket before saving
  - Reject upload with `400 Bad Request` if malware detected
  - Signature DB updates automatically via `freshclam`

- [ ] **Option B — Cloud API (VirusTotal free tier):**
  - VirusTotal API: 4 requests/minute on free tier
  - Upload file hash first (avoid uploading sensitive docs)
  - Fallback: scan only non-identity documents, or queue scan async

- [ ] Wire scanning into `WalletHandler.UploadDocument` before `fileStorage.Save()`

---

## Engineering Tasks (Future Sprints)

### P1 — Within 30 Days

#### 11. Frontend Wallet API Integration

Swap the frontend wallet store from direct Supabase calls to the new backend API.

- [ ] Create `src/features/wallet/api.ts` with functions for all 6 endpoints:
  - `uploadDocument(tripId, file, metadata)`
  - `listDocuments(tripId, filters)`
  - `getDocument(tripId, documentId)`
  - `deleteDocument(tripId, documentId)`
  - `serveFile(tripId, documentId)` (download)
  - `exportDocuments()` (future, see task 13)
- [ ] Update `src/utils/api-paths.ts` with wallet paths:
  ```typescript
  wallet: {
    documents: (tripId: string) => `/v1/trips/${tripId}/wallet`,
    document: (tripId: string, docId: string) => `/v1/trips/${tripId}/wallet/${docId}`,
    file: (tripId: string, docId: string) => `/v1/trips/${tripId}/wallet/${docId}/file`,
  }
  ```
- [ ] Refactor `src/features/wallet/store.ts` to use new API functions
- [ ] Wire up the wallet tab screen to the updated store
- [ ] Handle upload progress indicator (multipart upload)
- [ ] Test: upload, list, view, delete across Android and iOS

#### 12. Add In-App Consent Dialog

GDPR and general privacy best practice: inform users before storing sensitive documents.

- [ ] Show consent dialog on first wallet document upload
- [ ] Dialog content:
  - What data is stored (document file + metadata)
  - Where it is stored (encrypted server-side)
  - User can delete at any time
  - Special note for identity/health document categories
- [ ] Record consent timestamp (client-side in SecureStore initially)
- [ ] Skip dialog on subsequent uploads if consent already given
- [ ] Allow revoking consent from settings (deletes all wallet documents)

#### 13. Data Export Endpoint

For GDPR Right to Portability compliance.

- [ ] Implement `GET /v1/wallet/export`
- [ ] Returns a ZIP file containing:
  - All user documents (original files)
  - `manifest.json` with document metadata
- [ ] Rate-limit: 1 export per hour per user
- [ ] Stream ZIP generation to avoid high memory usage
- [ ] Add to frontend settings screen as "Export My Documents" button

---

### P2 — Within 90 Days

#### 14. Account Deletion Cascade

When a user deletes their account, all wallet data must be hard-deleted.

- [ ] Call `DeleteAllUserDocuments` during account deletion flow
- [ ] If no account deletion flow exists, create one:
  - Frontend: confirmation dialog with re-authentication
  - Backend: endpoint that cascades deletion across all tables
- [ ] Hard-delete files from storage (local or R2) — not just DB soft-delete
- [ ] Add to audit log before deletion (for compliance record)
- [ ] Test: delete account → verify zero files remain on disk and in DB

#### 15. WebSocket Events for Wallet

Publish real-time events so other clients update when documents change.

- [ ] Define event types in `internal/events/`:
  - `wallet.document.uploaded`
  - `wallet.document.deleted`
- [ ] Publish from wallet service layer after successful operations
- [ ] Frontend: subscribe in `WebSocketManager` and update Zustand wallet store
- [ ] Scope events to trip members only (existing WebSocket auth handles this)

#### 16. Image Thumbnails

Generate thumbnails for image uploads to improve list view performance.

- [ ] On upload of JPEG/PNG, generate a 200x200 thumbnail
- [ ] Store thumbnail alongside original (e.g., `<hash>_thumb.jpg`)
- [ ] Add `thumbnail_path` column to `wallet_documents` table (migration)
- [ ] Serve thumbnail in document list API response
- [ ] Frontend: display thumbnails in wallet document list
- [ ] Skip thumbnail generation for PDFs and non-image files (or use a placeholder icon)

---

## Priority Summary

| Priority | Tasks | Timeframe |
|---|---|---|
| **P0** | 1, 2, 3 (Coolify volume, EBS encryption, verify mount) | Before first real user upload |
| **P1** | 4, 5, 6, 7 (backups, monitoring, signing key, migrations) | Within 2 weeks |
| **P1** | 11, 12, 13 (frontend integration, consent, export) | Within 30 days |
| **P2** | 8, 9, 10 (R2 migration, disk resize, virus scan) | Before 1K users |
| **P2** | 14, 15, 16 (account deletion, WebSocket, thumbnails) | Within 90 days |
