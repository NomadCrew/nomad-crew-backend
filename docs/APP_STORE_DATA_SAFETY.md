# App Store Data Safety Guide — Wallet Feature Update

**Date:** 2026-02-17
**Purpose:** Guide for updating Google Play and Apple App Store data declarations after adding the wallet (travel document storage) feature.

---

## Google Play Data Safety Form

The Google Play Data Safety section must be updated in **Google Play Console > App content > Data safety** to reflect the new data types collected by the wallet feature.

### Updated data types

Items marked **NEW** must be added for the wallet feature. Items marked **existing** are already declared.

#### Personal info

| Data type | Collected? | Shared? | Purpose | Required? |
|---|---|---|---|---|
| Name | Yes (existing) | No | Account functionality | Yes |
| Email address | Yes (existing) | No | Account functionality | Yes |
| **Government ID (identity documents)** | **Yes (NEW)** | **No** | **App functionality (travel document storage)** | **No** |

#### Health info

| Data type | Collected? | Shared? | Purpose | Required? |
|---|---|---|---|---|
| **Health records (vaccination records)** | **Yes (NEW)** | **No** | **App functionality (travel document storage)** | **No** |

#### Financial info

| Data type | Collected? | Shared? | Purpose | Required? |
|---|---|---|---|---|
| **Other financial info (receipts)** | **Yes (NEW)** | **No** | **App functionality (travel expense receipts in wallet)** | **No** |

#### Files and docs

| Data type | Collected? | Shared? | Purpose | Required? |
|---|---|---|---|---|
| **Files and documents** | **Yes (NEW)** | **No** | **App functionality (travel document storage)** | **No** |

#### Location

| Data type | Collected? | Shared? | Purpose | Required? |
|---|---|---|---|---|
| Precise location | Yes (existing) | No | App functionality (trip location sharing) | No |

#### App activity

| Data type | Collected? | Shared? | Purpose | Required? |
|---|---|---|---|---|
| In-app search history | Yes (existing) | No | App functionality | No |
| Other user-generated content | Yes (existing) | No | App functionality (chat, todos) | No |

### Data handling declarations

For each **NEW** data type, the following answers apply in the Google Play form:

| Question | Answer |
|---|---|
| Is this data collected, shared, or both? | **Collected** |
| Is this data processed ephemerally? | **No** (stored on server until user deletes) |
| Is this data required for your app, or can users choose whether it's collected? | **Users can choose** (wallet upload is optional) |
| Why is this data collected? | **App functionality** |

### Data handling practices (global)

| Question | Answer |
|---|---|
| Is all collected data encrypted in transit? | **Yes** |
| Is all collected data encrypted at rest? | **Yes** |
| Do you provide a way for users to request that their data be deleted? | **Yes** |
| Committed to following Google Play Families Policy? | N/A (not a children's app) |

### Justification text for Google review

Use this text if Google requests additional justification for collecting sensitive data types:

> NomadCrew is a collaborative trip management app. The wallet feature allows users to optionally store travel documents (passports, visas, vaccination records, booking confirmations) for convenient access during trips. Document upload is entirely user-initiated and optional. Documents are encrypted at rest and in transit, accessible only to the uploading user (personal wallet) or trip members (group wallet with explicit user action). Users can delete their documents at any time. Health data (vaccination records) requires explicit user consent before upload.

---

## Apple App Store Privacy Labels

Privacy labels must be updated in **App Store Connect > App Privacy** to reflect the wallet feature.

### Data Linked to You

These are data types that are associated with the user's identity.

| Category | Data type | Purpose | Status |
|---|---|---|---|
| Contact Info | Email Address | App Functionality | Existing |
| Contact Info | Name | App Functionality | Existing |
| Location | Precise Location | App Functionality | Existing |
| Identifiers | User ID | App Functionality | Existing |
| **Sensitive Info** | **Government ID** | **App Functionality** | **NEW** |
| **Health & Fitness** | **Health Records** | **App Functionality** | **NEW** |
| **User Content** | **Photos** | **App Functionality** | **NEW** |
| **User Content** | **Files** | **App Functionality** | **NEW** |

### Data Not Linked to You

| Category | Data type | Purpose | Status |
|---|---|---|---|
| Diagnostics | Crash Data | Analytics | Existing |
| Diagnostics | Performance Data | Analytics | Existing |

### Data Used to Track You

**None.** NomadCrew does not track users across other companies' apps or websites.

### For each NEW data type, answer these Apple questions

#### Government ID (Sensitive Info)

| Question | Answer |
|---|---|
| Is this data used for tracking? | No |
| Is this data linked to the user's identity? | Yes |
| What are the purposes? | App Functionality |

#### Health Records (Health & Fitness)

| Question | Answer |
|---|---|
| Is this data used for tracking? | No |
| Is this data linked to the user's identity? | Yes |
| What are the purposes? | App Functionality |

#### Photos and Files (User Content)

| Question | Answer |
|---|---|
| Is this data used for tracking? | No |
| Is this data linked to the user's identity? | Yes |
| What are the purposes? | App Functionality |

### Justification text for Apple review

If Apple requests clarification during review, submit this explanation:

> NomadCrew's wallet feature allows travelers to optionally store copies of their travel documents (passports, visas, vaccination certificates, booking confirmations) within the app for quick reference during trips. This is a user-initiated convenience feature. No documents are collected automatically. Users choose which documents to upload and can delete them at any time. Health data (vaccination records) is collected only with explicit user consent. All documents are encrypted in transit and at rest. Government ID data is never used for identity verification, advertising, or any purpose beyond user-controlled storage and retrieval.

---

## Action Items Checklist

### Before wallet feature release

- [ ] Update Google Play Console > App content > Data safety form with new data types (Government ID, Health records, Financial info, Files and documents)
- [ ] Update Apple App Store Connect > App Privacy with new categories (Sensitive Info: Government ID, Health & Fitness: Health Records, User Content: Photos and Files)
- [ ] Prepare justification text for Apple review (see above)
- [ ] Prepare justification text for Google review (see above)
- [ ] Add in-app consent dialog before first wallet document upload (general documents)
- [ ] Add explicit consent dialog for health data uploads (vaccination records) -- must be separate and more specific than general consent
- [ ] Update the NomadCrew privacy policy to cover:
  - Types of documents stored
  - Encryption practices
  - Retention and deletion policies
  - Health data processing and explicit consent
  - User rights regarding stored documents
- [ ] Ensure privacy policy URL in both app stores points to updated version

### After wallet feature release

- [ ] Monitor Google Play Data Safety review for follow-up questions
- [ ] Monitor Apple App Review for privacy-related rejections
- [ ] Verify privacy labels are displayed correctly on both store listings
- [ ] Schedule 6-month DPIA review (see `WALLET_DPIA.md`)

---

## Reference Links

| Resource | URL |
|---|---|
| Google Play Data Safety help | https://support.google.com/googleplay/android-developer/answer/10787469 |
| Apple App Privacy Labels | https://developer.apple.com/app-store/app-privacy-details/ |
| NomadCrew DPIA | `docs/WALLET_DPIA.md` |
| ICO DPIA guidance | https://ico.org.uk/for-organisations/guide-to-data-protection/guide-to-the-general-data-protection-regulation-gdpr/data-protection-impact-assessments-dpias/ |
