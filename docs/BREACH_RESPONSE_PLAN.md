# Data Breach Response Plan

**NomadCrew -- Travel Document Wallet**
**Last updated:** 2026-02-17
**Owner:** NomadCrew Engineering Team
**Classification:** Internal -- All Team Members

---

## 1. Overview

NomadCrew's Travel Document Wallet stores sensitive personally identifiable information (PII) including:

- **Identity documents**: Passports, national IDs, visas
- **Health records**: Vaccination certificates, COVID test results
- **Travel documents**: Booking confirmations, insurance policies, itineraries

As a UK-based service, NomadCrew must comply with **UK GDPR** (Data Protection Act 2018). Key obligations:

| Requirement | Regulation | Deadline |
|---|---|---|
| Notify the ICO of qualifying breaches | Article 33 UK GDPR | **72 hours** from awareness |
| Notify affected users of high-risk breaches | Article 34 UK GDPR | **Without undue delay** |
| Document all breaches (even non-reportable ones) | Article 33(5) UK GDPR | Ongoing |

A "personal data breach" means any security incident leading to accidental or unlawful destruction, loss, alteration, unauthorised disclosure of, or access to, personal data.

---

## 2. Breach Classification

### Critical -- Mandatory ICO + User Notification

- Identity documents accessed or exfiltrated (passports, visas, national IDs)
- Signing keys compromised (WALLET_SIGNING_KEY, JWT_SECRET_KEY)
- Bulk export of user documents (any volume suggesting systematic access)
- Authentication system compromise (Supabase auth, JWT secrets)

**Action**: ICO notification required. User notification required under Article 34 (high risk to rights and freedoms).

### High -- ICO Notification Likely Required

- Personal travel documents exposed (bookings, insurance policies)
- Vaccination or health records accessed
- Single user's document store fully compromised
- Database backup accessed by unauthorised party

**Action**: ICO notification required unless you can demonstrate the breach is unlikely to result in risk to individuals (e.g., data was encrypted and keys were not compromised).

### Medium -- Assess Necessity

- Document metadata exposed (file names, document types, upload dates) without file content
- Access logs or audit trails leaked
- Unsuccessful but targeted attack on document storage

**Action**: Document internally. Assess whether metadata alone poses risk (document names may contain PII). Notify ICO if in doubt -- the ICO prefers over-reporting to under-reporting.

### Low -- Internal Documentation Only

- Automated vulnerability scan detected and blocked
- Failed authentication attempts (brute force blocked by rate limiting)
- Internal system breach with no evidence of data access
- Development/staging environment breach (no real user data)

**Action**: Log in incident register. No external notification unless assessment changes.

---

## 3. Detection Mechanisms

### Application-Level Monitoring

- **Audit log monitoring**: The `wallet_audit_log` table records every document access, download, and deletion. Alert on:
  - Single user downloading more than 10 documents in 5 minutes
  - Document access from a new IP address or device not previously associated with the user
  - Admin or service account accessing user documents outside of support workflows
  - Any `DELETE` operations on documents not initiated through the app UI

- **API anomalies**:
  - Spike in `/wallet/*` endpoint traffic
  - Requests to wallet endpoints from IPs outside normal geographic range
  - Authenticated requests using tokens for users who haven't been active recently

### Infrastructure-Level Monitoring

- Unauthorised SSH access attempts to EC2 instances
- Container escape or privilege escalation in Docker/Coolify
- Unusual outbound data transfer from application servers
- S3/storage bucket policy changes or public access modifications
- Database connection from unrecognised IP addresses

### External Reports

- User reports of unauthorised document access
- Notification from AWS of security event
- Third-party security researcher disclosure
- Law enforcement notification

---

## 4. Response Timeline

```
HOUR 0                                                    HOUR 72
  |--- Detection ---|--- Containment ---|--- Assessment ---|--- ICO ---|--- Users ---|
  0h              4h                  24h                48h          72h           72h+
```

### Phase 1: Detection and Initial Assessment (0-4 hours)

**Goal**: Confirm whether a breach has occurred, classify severity, begin containment.

| Time | Action | Who |
|---|---|---|
| 0h | Incident detected or reported | Anyone |
| 0-1h | Confirm breach is real (not false positive) | On-call engineer |
| 1-2h | Classify severity (Critical/High/Medium/Low) | On-call engineer |
| 2-3h | Notify team lead and data protection contact | On-call engineer |
| 3-4h | Begin evidence preservation (do NOT destroy logs) | On-call engineer |

**Key decision at 4h**: Is this Critical or High? If yes, activate full response. Start the 72-hour clock from the moment you have reasonable certainty a breach occurred.

### Phase 2: Containment and Scope (4-24 hours)

**Goal**: Stop ongoing data loss, determine what was accessed, preserve evidence.

| Time | Action | Who |
|---|---|---|
| 4-8h | Execute containment procedures (Section 8) | Engineering |
| 4-12h | Determine scope: which users, which documents, what time range | Engineering |
| 12-18h | Forensic review of audit logs, access logs, infrastructure logs | Engineering |
| 18-24h | Compile affected user list and data categories | Engineering + DPC |

### Phase 3: Impact Assessment and ICO Preparation (24-48 hours)

**Goal**: Determine notification obligations, prepare ICO submission.

| Time | Action | Who |
|---|---|---|
| 24-30h | Complete impact assessment: risk to affected individuals | DPC |
| 30-36h | Decision: is ICO notification required? | DPC + Team Lead |
| 36-42h | Draft ICO notification (Section 6) | DPC |
| 42-48h | Review and finalise ICO notification | Team Lead + DPC |

### Phase 4: Notification (48-72 hours)

| Time | Action | Who |
|---|---|---|
| 48-60h | Submit ICO notification via online portal | DPC |
| 60-72h | If Article 34 triggered: prepare and send user notifications | Engineering + DPC |

### Phase 5: Remediation (72+ hours)

| Time | Action | Who |
|---|---|---|
| 72h+ | Deploy technical fixes | Engineering |
| 1-2 weeks | Complete root cause analysis | Engineering |
| 2-4 weeks | Post-incident review and process updates | Full team |

---

## 5. Response Procedures

### On Discovery

1. **Do not panic. Do not cover up. Do not delete logs.**
2. Create an incident channel (Slack/Discord/WhatsApp group -- whatever the team uses)
3. Assign an **Incident Lead** (usually the most senior engineer available)
4. Start an incident log with timestamps for every action taken

### Evidence Preservation Checklist

- [ ] Screenshot or export relevant `wallet_audit_log` entries
- [ ] Export application server logs (`docker logs` from Coolify containers)
- [ ] Export AWS CloudWatch logs if applicable
- [ ] Save database query logs (PostgreSQL `pg_stat_activity`, slow query log)
- [ ] Record the current state of any compromised systems before making changes
- [ ] Note all IP addresses, user agents, and timestamps involved
- [ ] **Do not rotate credentials until evidence is preserved** (unless active exfiltration is ongoing, in which case containment takes priority)

### Scope Determination

Run these queries against the application database to determine impact:

```sql
-- Documents accessed in the incident timeframe
SELECT user_id, action, document_type, ip_address, created_at
FROM wallet_audit_log
WHERE created_at BETWEEN '[incident_start]' AND '[incident_end]'
ORDER BY created_at;

-- Count of affected users
SELECT COUNT(DISTINCT user_id)
FROM wallet_audit_log
WHERE created_at BETWEEN '[incident_start]' AND '[incident_end]';

-- Document types involved
SELECT document_type, COUNT(*)
FROM wallet_audit_log
WHERE created_at BETWEEN '[incident_start]' AND '[incident_end]'
GROUP BY document_type;
```

### Escalation Path

```
On-call engineer
  --> Team Lead (within 2 hours for Critical/High)
    --> Data Protection Contact (within 4 hours for Critical/High)
      --> External legal counsel (if breach affects >1000 users or involves law enforcement)
```

---

## 6. ICO Notification

### When to Notify

You **must** notify the ICO unless the breach is **unlikely to result in a risk to the rights and freedoms of individuals**. For NomadCrew's document wallet:

- Passport/visa/ID exposure: **Always notify** (identity theft risk)
- Health records exposure: **Always notify** (special category data)
- Encrypted documents exposed but keys safe: **Likely no notification** (but document the decision)
- Metadata only: **Assess case by case**

When in doubt, notify. The ICO will not penalise you for over-reporting.

### ICO Submission Portal

**URL**: https://ico.org.uk/for-organisations/report-a-breach/personal-data-breach/

**Phone** (for urgent breaches): 0303 123 1113

### Required Information

The ICO notification must include:

1. **Nature of the breach**: What happened (unauthorised access, data loss, etc.)
2. **Categories and approximate number of data subjects**: e.g., "approximately 150 users whose passport scans were stored"
3. **Categories and approximate number of records**: e.g., "approximately 200 passport images and 50 visa documents"
4. **Name and contact details of Data Protection Contact**
5. **Likely consequences**: Identity theft risk, financial fraud risk, etc.
6. **Measures taken or proposed**: Containment steps, user notification plans, remediation

### ICO Notification Template

```
PERSONAL DATA BREACH NOTIFICATION

Organisation: NomadCrew (travel management application)
Data Protection Contact: nomadcrew5@gmail.com
Date of breach: [DATE]
Date breach discovered: [DATE]
Reference: [INCIDENT-ID]

1. NATURE OF BREACH
[Describe what happened: e.g., "Unauthorised access to document storage via
compromised API credentials. An attacker accessed signed URLs for user-uploaded
travel documents between [DATE] and [DATE]."]

2. DATA SUBJECTS AFFECTED
- Number of individuals: [N] (approximate/confirmed)
- Categories: NomadCrew app users who uploaded travel documents

3. DATA CATEGORIES
- [ ] Passport scans
- [ ] Visa documents
- [ ] Vaccination records
- [ ] Booking confirmations
- [ ] Insurance documents
- [ ] Other: [specify]

4. LIKELY CONSEQUENCES
[e.g., "Affected users face risk of identity fraud as passport images contain
full name, date of birth, nationality, passport number, and photograph. No
financial data (payment cards, bank details) was involved."]

5. MEASURES TAKEN
- [e.g., "All active signed URLs revoked immediately upon discovery"]
- [e.g., "Document signing key rotated"]
- [e.g., "Affected users notified via push notification and email"]
- [e.g., "Forensic review of access logs completed"]

6. MEASURES PROPOSED
- [e.g., "Implementing additional access controls on document download endpoints"]
- [e.g., "Adding IP-based anomaly detection to audit log monitoring"]
```

---

## 7. User Notification

### When to Notify Users

Under Article 34, user notification is required when a breach is "likely to result in a **high risk** to the rights and freedoms of natural persons." For identity documents, this threshold is almost always met.

### Notification Channels

Send via **all available channels** simultaneously:

1. **Push notification**: Immediate alert with link to detailed information
2. **Email**: Full details to the email address on file
3. **In-app banner**: Persistent banner shown on next app open

### Push Notification Text

```
Security Notice: Your NomadCrew account may have been affected by a data
incident. Please open the app for important information and recommended actions.
```

### Email / In-App Notification Template

```
Subject: Important Security Notice from NomadCrew

Dear [USER],

We are writing to inform you of a security incident that may affect your
NomadCrew account.

WHAT HAPPENED
[Plain language description: e.g., "On [DATE], we detected unauthorised
access to our document storage system. An unauthorised party may have
accessed travel documents uploaded to the NomadCrew wallet feature."]

WHAT DATA WAS INVOLVED
Based on our investigation, the following types of documents stored in your
wallet may have been accessed:
- [List specific document types for this user]

WHAT WE ARE DOING
- We immediately revoked all access to stored documents
- We have notified the UK Information Commissioner's Office
- We are conducting a full investigation with [external security firm if applicable]
- [Other remediation steps]

WHAT YOU SHOULD DO
- Monitor your accounts for unusual activity
- If passport data was involved: consider contacting HM Passport Office
  (0300 222 0000) about flagging your passport
- If vaccination records were involved: no immediate action required, but
  be alert to phishing attempts referencing your health data
- Be cautious of emails or messages claiming to be from NomadCrew -- we
  will never ask for your password or payment details via email
- Consider signing up for a fraud monitoring service

CONTACT US
If you have questions, contact us at nomadcrew5@gmail.com.

We sincerely apologise for this incident and are committed to protecting
your data.

NomadCrew Team
```

---

## 8. Containment Procedures

Execute these in order. Skip steps that don't apply to the specific incident.

### Immediate (within 1 hour of confirmed breach)

```bash
# 1. If active exploitation: disable wallet endpoints
# Add to Nginx/Coolify config or application feature flag
# This blocks all document upload/download/access

# 2. Revoke all active signed URLs by rotating the signing key
# Update WALLET_SIGNING_KEY in Coolify environment variables
# Redeploy the backend service -- all existing signed URLs become invalid

# 3. If auth compromise suspected: rotate JWT secrets
# Update JWT_SECRET in Coolify environment variables
# This will force all users to re-authenticate

# 4. If database compromise suspected: rotate database credentials
# Update in Coolify PostgreSQL configuration
# Update connection strings in backend service
```

### Short-term (within 24 hours)

- [ ] Block attacker IP addresses at infrastructure level (AWS security groups)
- [ ] Revoke any compromised API keys or service account credentials
- [ ] Review and restrict IAM/access policies that were overly permissive
- [ ] Enable additional logging if not already active
- [ ] Verify backup integrity (ensure backups weren't tampered with)

### Verification

- [ ] Confirm no ongoing unauthorised access in audit logs
- [ ] Verify rotated credentials are working correctly
- [ ] Test that legitimate users can still access the service (after re-auth if needed)
- [ ] Confirm monitoring/alerting is active for the affected systems

---

## 9. Post-Incident

### Root Cause Analysis

Complete within **2 weeks** of incident resolution. Document:

1. **Timeline**: Exact sequence of events from initial compromise to detection to resolution
2. **Root cause**: The specific vulnerability or failure that enabled the breach
3. **Contributing factors**: Process gaps, missing monitoring, delayed detection
4. **Detection time**: How long between breach and detection? Why not sooner?

### Remediation Actions

For each finding, create a tracked task with:
- Description of the fix
- Owner
- Deadline
- Verification criteria

### Process Updates

- Update this breach response plan with lessons learned
- Update monitoring and alerting based on detection gaps
- Update access controls and security configurations
- Review and update data retention policies

### Team Retro

Hold within **1 week** of incident closure. Blameless format:
- What went well in the response?
- What could have gone better?
- What will we change?

Document outcomes and action items.

---

## 10. Contact Information

| Role | Contact | When to Reach |
|---|---|---|
| Data Protection Contact | nomadcrew5@gmail.com | All breaches Critical/High |
| ICO (Information Commissioner's Office) | 0303 123 1113 | Critical/High breaches within 72h |
| ICO Breach Report Portal | https://ico.org.uk/for-organisations/report-a-breach/ | Formal breach notification |
| AWS Support | Via AWS Console | Infrastructure compromise |
| Hosting Region | AWS eu-west (Ireland) | Reference for ICO notification |
| Supabase Support | Via Supabase dashboard | Auth system compromise |

### External Resources

- [ICO Breach Reporting Guide](https://ico.org.uk/for-organisations/report-a-breach/)
- [ICO Personal Data Breach Assessment Tool](https://ico.org.uk/for-organisations/report-a-breach/personal-data-breach-assessment/)
- [NCSC Incident Management Guidance](https://www.ncsc.gov.uk/collection/incident-management)

---

## Appendix: Incident Log Template

Use this template to record every breach, regardless of severity.

```
INCIDENT LOG

ID: INC-[YYYY]-[NNN]
Severity: [Critical / High / Medium / Low]
Status: [Open / Contained / Resolved / Closed]

Detection
- Date/time detected:
- Detected by: [monitoring / user report / external]
- Initial classifier: [name]

Assessment
- Date/time confirmed:
- Data categories involved:
- Number of users affected:
- Breach classification:

Response
- Containment actions taken:
- ICO notified: [Yes / No / Not required]
- ICO reference number:
- Users notified: [Yes / No / Not required]
- Date users notified:

Resolution
- Root cause:
- Remediation actions:
- Date resolved:
- Post-incident review date:
- Lessons learned:
```

---

*This plan should be reviewed and updated at least annually or after any breach incident, whichever comes first.*
