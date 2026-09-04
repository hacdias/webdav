# CLAUDE.md

Guidance for Claude when working in this repository (`hacdias/webdav`).

## Handling security advisories

Advisory state lives on GitHub and is driven with the `gh` CLI: `triage → draft → published`, plus `closed`. Reports are routed as per [SECURITY.md](../SECURITY.md); only `5.x` is supported.

### 1. Fetch

```bash
# List by state (also: published, draft, closed)
gh api '/repos/hacdias/webdav/security-advisories?state=triage&per_page=100' \
  --jq '.[] | {ghsa_id, severity, summary, state}'

# Full report for one advisory
gh api /repos/hacdias/webdav/security-advisories/GHSA-xxxx-xxxx-xxxx \
  --jq '.summary, "---", .description'
```

Always pull the published and remaining triage sets too, to dedup against.

### 2. Verify — do NOT trust the report text

Read the source at HEAD and reproduce the claim; a failing `makeTestServer` case is better evidence than reading the matcher. Reach one verdict per advisory:

- **CONFIRMED** — defect exists at HEAD. Quote the exact `file:line`.
- **FIXED** — already patched; find the fix commit and the release carrying it.
- **FALSE / NOT APPLICABLE** — claim is wrong, or targets a different project.
- **NOT EXPLOITABLE** — pattern exists but no code path reaches the precondition.
- **DUPLICATE** — of a published advisory, or of another triage advisory.

Common traps:

- **"Incomplete fix of a prior advisory."** Read the original fix commit and confirm the specific sibling path is still unguarded. `GHSA-chxv-mvjv-f92j` already cleans paths in `newRequest` and matches trailing-slash rules against the bare collection.
- **Wrong project.** Confirm the cited files, symbols, and options exist here — reports sometimes describe a fork or another WebDAV server.
- **Containment vs authorization.** `golang.org/x/net/webdav` applies `slashClean` and keeps requests inside the served root, so "traversal out of `directory`" is usually not the defect. The real class is the authorization layer disagreeing with the filesystem layer about which file a request names.
- **No `users:` means no authentication, by design** — it warns at startup. Not a vulnerability, but it changes the privilege precondition.
- **Overlapping reports.** Several triage advisories may share one root cause: consolidate into one, close the rest as duplicates.

Record per advisory: verdict, `file:line` evidence, preconditions (default config? needs `directories:`? platform-specific? auth required?), disposition.

### 3. Severity

Set a CVSS v3.1 vector — GitHub derives the score and severity from it, overriding the plain `severity` field. Encode the real preconditions so the band is defensible: a required configuration or platform (case-insensitive filesystem, `directories:`) is **AC:H**; needing an account is **PR:L**. Rate the base case as a config with a `users:` block, and note the unauthenticated vector in the body when it is materially worse. Don't let an incomplete-fix follow-up outrank its parent.

```bash
gh api -X PATCH /repos/hacdias/webdav/security-advisories/GHSA-xxxx-xxxx-xxxx \
  -f cvss_vector_string='CVSS:3.1/AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:N' \
  --jq '{ghsa_id, severity, score: .cvss.score, vector: .cvss.vector_string}'
```

### 4. Rewrite the title and body

The title is `summary`: concise and sentence-case, stating the vulnerability class then the mechanism, e.g. `Authorization bypass: path rules can be evaded with dot segments or a bare collection name`.

Rewrite `description` into the sections below, reusing the reporter's own wording where it is accurate. Drop the greeting and anything step 2 disproved. Use `###` headings, keep this order, omit what doesn't apply. Keep the maintainer's voice — first person belongs only in quoted PoC steps.

| Section            | Contents                                                                                                       |
| ------------------ | -------------------------------------------------------------------------------------------------------------- |
| `Summary`          | The defect and its root cause, naming the file and function, quoting the pre-fix code.                         |
| `Impact`           | Who can exploit it and what they get. State what is *not* affected.                                            |
| `Proof of concept` | Config and steps trimmed to the essentials, with observed results.                                             |
| `Patches`          | `Fixed in **vX.Y.Z**. Upgrade to that version or later.` plus what the fix does and why it sits where it does. |
| `Workarounds`      | What the operator can do themselves. `None.` if nothing helped, saying why.                                    |
| `Out of scope`     | What the report claimed that is deliberately not treated as a vulnerability, and why.                          |
| `References`       | Related issues, commits, published advisories.                                                                 |

Send it as a file so the Markdown survives shell quoting:

```bash
jq -Rs '{description: .}' desc.md \
  | gh api -X PATCH .../security-advisories/GHSA-xxxx-xxxx-xxxx --input -
```

### 5. Affected versions

The package is always `{ecosystem: "go", name: "github.com/hacdias/webdav/v5"}`. `vulnerable_version_range` ends at the last release before the fix; add a lower bound when the defect was introduced in a known version, confirming with `git log -S` and `git tag --contains`. `patched_versions` is the release carrying the fix.

```bash
printf '%s' '{"vulnerabilities":[{"package":{"ecosystem":"go","name":"github.com/hacdias/webdav/v5"},"vulnerable_version_range":">= 5.10.0, <= 5.14.1","patched_versions":"5.14.2","vulnerable_functions":[]}]}' \
  | gh api -X PATCH .../security-advisories/GHSA-xxxx-xxxx-xxxx --input -
```

### 6. Move state

```bash
gh api -X PATCH .../security-advisories/GHSA-xxxx-xxxx-xxxx -f state=draft    # ready to publish
gh api -X PATCH .../security-advisories/GHSA-xxxx-xxxx-xxxx -f state=closed   # duplicate / N-A / not-exploitable
```

- **CONFIRMED** → fix, release, set `patched_versions` → draft, then publish once the release is out.
- **DUPLICATE / NOT APPLICABLE / NOT EXPLOITABLE** → closed.

The REST API cannot post advisory comments. Replies to reporters must be posted manually in the UI — draft the text for the maintainer.
