# Data Protection Impact Assessment (DPIA) — Wallet Document Storage

**Feature:** NomadCrew Wallet (travel document upload and storage)
**Date:** 2026-02-17
**Author:** NomadCrew Compliance
**Status:** Draft
**Review Date:** 6 months from wallet feature launch
**Applicable Regulations:** UK GDPR, EU GDPR, Data Protection Act 2018

---

## 1. Description of Processing

### 1.1 What personal data is processed?

The wallet feature allows users to upload, store, and retrieve travel-related documents within the NomadCrew mobile application. The following personal data is processed:

| Data Category | Examples | GDPR Classification |
|---|---|---|
| Identity documents | Passports, national ID cards, visa pages | Personal data (may include biometric photos) |
| Travel documents | Booking confirmations, boarding passes, hotel reservations | Personal data |
| Insurance documents | Travel insurance policies, coverage cards | Personal data |
| Vaccination records | COVID-19 certificates, yellow fever cards, immunization records | **Special category data (health data, Article 9)** |
| Financial receipts | Expense receipts, payment confirmations | Personal data |
| Document metadata | File name, document type label, validity start/end dates, upload timestamp, file size | Personal data |

**File formats accepted:** PDF, JPEG, PNG, HEIC

### 1.2 Why is the data processed?

The purpose is **travel convenience** -- enabling users to centralize their travel documents in one secure location for easy access during trips. This eliminates the need to carry physical copies or search through email for confirmations.

Specific use cases:

- Quick access to passport details at airport check-in or border control
- Sharing booking confirmations with trip members (group wallet)
- Keeping vaccination records accessible for countries requiring proof
- Storing receipts for group expense reconciliation

### 1.3 Who are the data subjects?

- **Primary:** International travelers using the NomadCrew app
- **Geographic focus:** EU/UK users (GDPR applies), with a global user base
- **Age:** Adults (18+); the app does not target minors
- **Volume:** Expected low-to-moderate volume at launch (MVP), scaling with user growth

### 1.4 How is the data processed?

```
User selects document on mobile device
    |
    v
Mobile app captures file + metadata (name, type, dates)
    |
    v
HTTPS upload (TLS 1.2+) to Go backend API
    |
    v
Backend validates file (type, size), generates signed filename
    |
    v
File stored on encrypted server storage (EBS volume, AES-256)
    |
    v
Metadata persisted to PostgreSQL (Coolify-hosted)
    |
    v
Retrieval via signed URLs (15-minute expiry, single-use intent)
```

**Data flows:**

- **Upload:** Mobile device --> HTTPS --> Go backend --> encrypted disk + PostgreSQL
- **Retrieval:** Mobile device --> HTTPS --> Go backend --> signed URL --> encrypted disk --> HTTPS --> mobile device
- **Deletion:** User triggers soft-delete --> 30-day retention --> hard-delete (file + metadata)

### 1.5 Third parties and processors

| Party | Role | Data Access | Basis |
|---|---|---|---|
| AWS (EC2, EBS) | Infrastructure provider (processor) | Encrypted storage at rest | DPA in place via AWS standard terms |
| Supabase | Authentication provider (processor) | User ID and auth tokens only; **no document data** | DPA in place via Supabase terms |
| Expo / EAS | Build and update service | No access to user documents | N/A |
| Coolify | Self-hosted deployment platform | PostgreSQL metadata only (on NomadCrew-controlled infra) | Self-hosted; no third-party access |

### 1.6 Data retention

| Stage | Duration | Mechanism |
|---|---|---|
| Active storage | Until user deletes or trip ends (user-controlled) | User action via app |
| Soft-deleted | 30 days | `deleted_at` timestamp; file not served |
| Hard-deleted | After 30-day soft-delete window | Background purge job removes file from disk and metadata from DB |
| Account deletion | All documents purged within 30 days of account deletion | Cascading delete process |

---

## 2. Necessity and Proportionality

### 2.1 Lawful basis for processing

| Data Type | Lawful Basis | GDPR Article | Justification |
|---|---|---|---|
| Identity documents, travel docs, receipts | **Consent** | Article 6(1)(a) | User voluntarily chooses to upload each document. No documents are collected automatically. |
| Vaccination records (health data) | **Explicit consent** | Article 9(2)(a) | Health data requires explicit consent under Article 9. A specific consent dialog is presented before the first health-related document upload. |

**Why consent (not legitimate interest)?**

Document storage is an optional convenience feature, not core to the app's primary function (trip coordination). Users who never upload documents can still use all other NomadCrew features. Consent is the most appropriate and transparent basis.

### 2.2 Data minimization

- **User-initiated only:** The app never automatically collects, scans, or extracts data from documents. Every upload is a deliberate user action.
- **No OCR or parsing:** Documents are stored as opaque files. The backend does not extract text, passport numbers, or any structured data from uploaded files.
- **Metadata is user-provided:** The user enters the document name, type, and optional validity dates. No metadata is inferred or scraped.
- **No cross-referencing:** Document data is not used for profiling, analytics, advertising, or any purpose beyond storage and retrieval.

### 2.3 Storage limitation

- Users control their own documents and can delete at any time.
- Per-user storage quotas prevent unbounded accumulation (100MB personal, 500MB group per trip).
- 30-day post-deletion purge ensures data is not retained indefinitely after the user's intent to delete.
- No indefinite retention -- documents are tied to active user accounts and active trips.

### 2.4 Purpose limitation

Document data is used **exclusively** for:

1. Storing the document for later retrieval by the uploading user
2. Sharing the document with trip members (group wallet only, with user's explicit action)

Document data is **never** used for:

- Analytics or profiling
- Advertising or marketing
- Training machine learning models
- Sharing with third parties
- Identity verification by NomadCrew

### 2.5 Data subject rights

| Right | Implementation |
|---|---|
| Access (Art. 15) | Users can view and download all their documents in-app |
| Rectification (Art. 16) | Users can re-upload corrected documents and delete old versions |
| Erasure (Art. 17) | Users can delete individual documents or request full account deletion |
| Restriction (Art. 18) | Soft-delete mechanism effectively restricts processing |
| Portability (Art. 20) | Documents are stored in original format (PDF, JPEG, etc.) and downloadable |
| Objection (Art. 21) | N/A (consent-based; users withdraw consent by deleting documents) |
| Withdraw consent | Users can delete documents at any time; no penalty or loss of other features |

---

## 3. Risks Identified

### 3.1 Risk register

| # | Risk | Likelihood | Severity | Overall | Description |
|---|---|---|---|---|---|
| R1 | Unauthorized access to identity documents | Medium | High | **High** | An attacker gains access to stored passport/visa files, enabling identity theft or fraud |
| R2 | Data breach exposing document files | Low | Critical | **High** | Server compromise or misconfiguration exposes stored files to unauthorized parties |
| R3 | Group wallet oversharing | Medium | Medium | **Medium** | Trip members see documents shared to the group wallet that the uploader did not intend to share widely |
| R4 | Server/infrastructure compromise | Low | Critical | **High** | Compromise of the EC2 instance or EBS volume exposes all stored files |
| R5 | Third-party infrastructure risk | Low | High | **Medium** | AWS service compromise or misconfiguration exposes data |
| R6 | Malicious file upload | Medium | Medium | **Medium** | User uploads a file containing malware that could affect other users or infrastructure |
| R7 | Insider threat | Low | High | **Medium** | A team member with server access views or exfiltrates document files |
| R8 | Loss of data availability | Low | Medium | **Low** | Server failure causes loss of stored documents |
| R9 | Metadata leakage | Low | Medium | **Low** | Document metadata (names, types, dates) exposed through API responses or logs |
| R10 | Consent validity for health data | Medium | High | **High** | Explicit consent mechanism for health data (Article 9) not yet implemented in the app. Must be built before wallet launch. |

### 3.2 Risk scoring methodology

- **Likelihood:** Low / Medium / High
- **Severity:** Low / Medium / High / Critical
- **Overall:** Derived from the combination, with severity weighted more heavily due to the sensitivity of identity and health documents

---

## 4. Risk Mitigation Measures

### R1: Unauthorized access to identity documents

| Measure | Status |
|---|---|
| JWT-based authentication on all wallet endpoints | Implemented |
| RBAC enforcement: personal documents accessible only by the owning user | Implemented |
| RBAC enforcement: group documents accessible only by active trip members | Implemented |
| Signed URLs with 15-minute expiry for file retrieval | Implemented |
| Soft-delete check before serving any file (deleted files return 404) | Implemented |
| Separate `WALLET_SIGNING_KEY` (not reusing other signing secrets) | Implemented |
| Security headers on file responses (`Content-Disposition: attachment`, `Cache-Control: no-store`) | Implemented |

**Residual risk after mitigation:** Low

### R2: Data breach exposing document files

| Measure | Status |
|---|---|
| Encryption at rest via AWS EBS volume encryption (AES-256) | Implemented |
| Encryption in transit via TLS 1.2+ (HTTPS only) | Implemented |
| Files stored with non-guessable names (UUID-based paths) | Implemented |
| No directory listing or file enumeration possible via API | Implemented |
| Audit logging of all document access events | Implemented |

**Residual risk after mitigation:** Low

### R3: Group wallet oversharing

| Measure | Status |
|---|---|
| Clear separation between personal wallet and group (trip) wallet | Implemented |
| User must explicitly choose to share a document to the group wallet | Implemented |
| In-app warning shown before sharing a document to a group | Planned |
| Group wallet documents visible only to active trip members (not removed/left members) | Implemented |

**Residual risk after mitigation:** Low (user education reduces accidental sharing)

### R4: Server/infrastructure compromise

| Measure | Status |
|---|---|
| EBS encryption protects data if physical media is compromised | Implemented |
| EC2 instance runs in private VPC with restricted security groups | Implemented |
| SSH access restricted to authorized keys only | Implemented |
| Docker containerization isolates application processes | Implemented |
| Planned migration to R2/S3 with bucket-level encryption and IAM policies | Planned (post-MVP) |

**Residual risk after mitigation:** Medium (local filesystem storage is MVP trade-off)

### R5: Third-party infrastructure risk

| Measure | Status |
|---|---|
| AWS DPA covers data processing obligations | In place |
| Supabase has no access to document content (auth-only) | By architecture |
| No document data shared with any third party | By design |
| Infrastructure monitoring and alerting | Implemented |

**Residual risk after mitigation:** Low

### R6: Malicious file upload

| Measure | Status |
|---|---|
| File type validation (only PDF, JPEG, PNG, HEIC accepted) | Implemented |
| File size limits enforced (10MB per file) | Implemented |
| Per-user storage quotas (100MB personal, 500MB group) | Implemented |
| Files served with `Content-Disposition: attachment` (prevents browser execution) | Implemented |
| Virus/malware scanning on uploads | **Not implemented** (planned for future) |

**Residual risk after mitigation:** Medium (no virus scanning is a known gap)

### R7: Insider threat

| Measure | Status |
|---|---|
| Minimal team with server access | In place |
| Audit logging of file access at the application level | Implemented |
| EBS encryption means raw disk access does not expose plaintext | Implemented |
| Planned: application-level encryption of file contents (envelope encryption) | Planned (post-MVP) |

**Residual risk after mitigation:** Medium

### R8: Loss of data availability

| Measure | Status |
|---|---|
| Users retain original documents on their devices | By design (app stores copies, not originals) |
| In-app messaging: wallet is a convenience copy, not a primary backup | Planned |
| Planned: R2/S3 migration with cross-region replication | Planned (post-MVP) |

**Residual risk after mitigation:** Low (users are not expected to rely on the app as sole storage)

### R9: Metadata leakage

| Measure | Status |
|---|---|
| API responses contain only necessary metadata fields | Implemented |
| Document metadata not included in application logs | Implemented |
| Signed URL paths do not contain document names or types | Implemented |

**Residual risk after mitigation:** Low

### R10: Consent validity for health data

| Measure | Status |
|---|---|
| Explicit consent dialog before first health document upload (separate from general terms) | Planned |
| Consent records stored with timestamp and version | Planned |
| Users can withdraw consent by deleting health documents | Implemented |
| Privacy policy updated to specifically address health data processing | Planned |

**Residual risk after mitigation:** Low (once consent mechanism is implemented)

---

## 5. Residual Risks Summary

| Risk | Pre-Mitigation | Post-Mitigation | Remediation Plan | Timeline |
|---|---|---|---|---|
| R1: Unauthorized access | High | **Low** | Maintain current controls | Ongoing |
| R2: Data breach | High | **Low** | Maintain current controls | Ongoing |
| R3: Group oversharing | Medium | **Low** | Add in-app warning before group sharing | v1.1 |
| R4: Server compromise | High | **Medium** | Migrate to R2/S3 with IAM policies | Post-MVP (Q3 2026) |
| R5: Third-party risk | Medium | **Low** | Review DPAs annually | Annual |
| R6: Malicious uploads | Medium | **Medium** | Integrate virus scanning (ClamAV or cloud-based) | Post-MVP (Q2 2026) |
| R7: Insider threat | Medium | **Medium** | Add application-level envelope encryption | Post-MVP (Q3 2026) |
| R8: Data loss | Low | **Low** | Migrate to R2/S3 with replication | Post-MVP (Q3 2026) |
| R9: Metadata leakage | Low | **Low** | Maintain current controls | Ongoing |
| R10: Consent validity | **High** | **Low** (after mitigation) | Implement explicit consent flow for health data with separate, informed consent dialog for Article 9 special category data | **Before launch (BLOCKER)** |

### Key residual risks requiring attention before production launch

1. **Explicit consent mechanism for health data (R10):** Must be implemented before the wallet feature goes live to meet Article 9 requirements.
2. **Virus scanning (R6):** Accepted as a medium residual risk for MVP, with a committed timeline for remediation.
3. **Local filesystem storage (R4):** Accepted for MVP with the understanding that R2/S3 migration is prioritized.

---

## 6. Conclusion

### Assessment outcome

The processing of personal data through the NomadCrew wallet feature is **necessary and proportionate** for the stated purpose of travel document convenience. The feature provides genuine utility to travelers and processes only data that users voluntarily upload.

### Key findings

1. **Lawful basis is appropriate.** Consent (Article 6(1)(a)) is the correct basis for an optional convenience feature. Explicit consent (Article 9(2)(a)) is required and planned for health data.

2. **Technical controls are adequate for MVP.** Encryption at rest and in transit, RBAC, signed URLs, audit logging, and storage quotas collectively reduce risk to acceptable levels for initial launch.

3. **Three medium residual risks are documented** with specific remediation timelines:
   - Virus scanning: Q2 2026
   - R2/S3 migration (replacing local filesystem): Q3 2026
   - Application-level encryption: Q3 2026

4. **One pre-launch requirement remains:** The explicit consent mechanism for health data must be implemented before the wallet feature is made available to users.

### Approval and review

| Item | Detail |
|---|---|
| DPIA completed | 2026-02-17 |
| Next review date | 6 months from wallet feature launch |
| Review triggers (before scheduled date) | Security incident, significant architecture change, new data types, regulatory guidance update |
| DPO consultation required? | Yes, if processing health data at scale. At MVP scale, this DPIA is sufficient. |

### Sign-off

| Role | Name | Date | Signature |
|---|---|---|---|
| Data Controller | ___________________ | __________ | __________ |
| Technical Lead | ___________________ | __________ | __________ |
| DPO (if applicable) | ___________________ | __________ | __________ |
