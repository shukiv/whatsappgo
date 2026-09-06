# Bug reporting

WhatsAppGo sends reports to `https://bugs.jabali-panel.com/api/v1/intake`, with
`program: "whatsappgo"` and `source: "whatsappgo"`. The intake routes them to
the `WHATSAPPGO` project in Plane's `bug-reports` workspace. Reports no longer
use `gh issue create`; GitHub authentication is not required. Release updates
still come from the GitHub repository.

## Configure an intake key

Ask the intake operator for a key issued for WhatsAppGo. Do not embed a shared
key in source code, a packaged binary, screenshots, or bug descriptions.

The app-owned daemon reads these runtime variables:

| Variable | Meaning |
| --- | --- |
| `WHATSAPPGO_BUGREPORT_TOKEN_FILE` | Absolute path to a UTF-8 file containing only the intake key; surrounding whitespace is trimmed |
| `WHATSAPPGO_BUGREPORT_TOKEN` | Direct key value; takes precedence over the file when nonempty |

Prefer an owner-readable file (`0600`) in a private directory, outside the
repository. On Windows, restrict its ACL to the current user. Set the file
variable in the desktop launch environment so its backend processes inherit it:

```bash
WHATSAPPGO_BUGREPORT_TOKEN_FILE=/absolute/path/to/intake-key ./desktop/build/whatsappgo
```

Quit an already-running desktop normally before relaunching with new environment
variables. Merely opening another window does not reconfigure an existing
backend. The key file is read on each submission, so replacing its contents
with a newly issued key does not require another restart.

Missing, unreadable, empty, or invalid keys produce an actionable error without
discarding the report text. No fallback to GitHub or anonymous submission occurs.

## Submit a report

Open **Report a problem**, describe the steps, actual behavior, and expected
behavior, review the environment block, then choose **Send report**. This is an
explicit submission, not automatic telemetry or automatic upload of chat logs.

Automation can use the same desktop-owned backend:

```bash
whatsappctl call bugreport.environment '{}'
# This next command files a real report; use it only for an actual finding.
whatsappctl call bugreport.submit '{"subject":"Short bug title","body":"Reproduction steps, expected result, actual result"}'
```

The local RPC accepts only `subject` and `body`; callers cannot override the
destination, project, or authentication. The daemon sends JSON containing the
title, description with the safe environment block appended, program, source,
and default severity `medium`. It does not add logs, a reporter identity,
screenshots, chat identifiers, or a deduplication fingerprint. Each user-created
report therefore normally creates a new intake item.

The title is whitespace-folded and bounded to 120 UTF-8 bytes; user descriptions
are bounded to 8,000 bytes without splitting a character. The appended environment
contains version, OS, architecture, Go version, connection/login booleans,
account count, and uptime—not account names, phone numbers, messages, or keys.
Anything manually typed in the description does leave this device: omit secrets
and private chat data even though the intake also performs redaction.

## Responses and retries

An authenticated POST must return HTTP `201` (created) or `200` (commented),
`ok: true`, the `whatsappgo` program, a recognized action, and an HTTP(S) item
URL before the app reports success. The URL may be a LAN address supplied by
Plane; opening it may require access to that network. Non-fatal intake warnings
do not turn an already-created report into a failed submission.

The client refuses redirects so neither credentials nor report bodies move to
another endpoint. Responses are bounded to 64 KiB and requests to 100 seconds.
There are no automatic POST retries. Authentication failures ask for a valid key;
rate limits ask the user to wait. A timeout or malformed success response may
mean the item was created but its acknowledgement was lost: check the intake
before sending the same report again.

## Verification and corrected findings (2026-09-06)

These findings were fixed locally and regression-tested. This entry is not a
claim that live intake tickets have been created; authenticated live submission
requires the operator-provided key.

| Finding | Fix and regression |
| --- | --- |
| High: identity consolidation could resurrect deleted messages or lose edits | Merge tombstones and edited revisions in both directions; `internal/store/alias_merge_test.go` |
| High: clearing cached media made pre-merge attachments unreachable | Restore from canonical and historical JID archive keys, including already-merged histories; `internal/whatsapp/media_alias_test.go` |
| High: failed sends discarded text, reply context, and pasted images | Consume only acknowledged drafts; retain images/captions on failure and scope completions to the original account/chat; `desktop-composer-drafts`, `desktop-conversation-updates` |
| Medium: identity consolidation dropped rich message metadata | Preserve stars, preview cards, durations, waveforms, receipt times, contact/location and forwarding data; `internal/store/alias_merge_test.go` |
| Medium: Mark all read processed only the first 100 active/archived chats | Page both lists in valid 500-chat batches, collect targets before changing state, and continue after individual failures; `internal/whatsapp/chatsettings_test.go` |

Intake tests use local HTTP servers, not the production tracker. They cover the
program, authentication, payload, successful responses, rejected/invalid
responses, redirects, cancellation, key-file loading, and UTF-8 limits. The
service test checks destination disclosure and error propagation.

The maximum-history retention policy remains unchanged: disappearing-message
timers do not expire local conversation history. Explicit revocations and
deletion actions remain separate. See [the retention policy](ARCHITECTURE.md#local-history-retention-policy).
