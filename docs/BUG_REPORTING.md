# Bug reporting

## Desktop reporting

**Report a problem** in the toolbar and Help opens
<https://github.com/shukiv/whatsappgo/issues> directly in the default browser.
There is no intermediate dialog, in-app submission, or intake key requirement,
even if a key is configured. The browser controls tab versus window placement.
If launching fails, the app displays the URL for manual navigation.

Opening the page sends no app data, environment block, logs, or report text and
does not create an issue. The user reviews or files issues on GitHub themselves.

## Optional intake tooling (not the desktop reporting action)

The existing local `bugreport.*` RPC and authenticated Jabali intake client are
retained for separately configured tooling. They are not called by the desktop
report buttons. The sections below document that optional interface and the
earlier intake flow; the desktop now always uses GitHub issues.

## Hosted intake alternative (no key)

Visit `https://bugs.jabali-panel.com/report` manually in your browser if you
specifically need the intake instead of GitHub. Choose **whatsappgo**
in the program field, describe the problem, complete the Turnstile security
check, and submit there. The form does not preselect a program from the URL.
Do not automate this public form.

Opening the page does not submit a report or upload app data. No report text,
environment, or key is passed in the browser URL. The website records the
submitted report, IP address, and browser
type. Public submissions are labeled `public` and `bug` by the intake.

This alternative is not opened by the desktop's report actions.

## Configure an intake key (optional)

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

Missing, unreadable, empty, or malformed local keys prevent RPC submission.
Presence of a readable key is not proof the server will accept it: rejected keys
produce an actionable RPC submission error. The RPC never silently submits
anonymously or files an issue on GitHub.

## Authenticated reporting

Tooling can explicitly submit through the desktop-owned backend after reviewing
the environment block. This is not automatic telemetry or a chat-log upload:

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

`bugreport.environment` also returns the fixed `public_url` and an
`authenticated_available` boolean. The boolean reveals only whether a local
credential can be read and passes format validation, never its value or path.

The title is whitespace-folded and bounded to 120 UTF-8 bytes; user descriptions
are bounded to 8,000 bytes without splitting a character. The appended environment
contains version, OS, architecture, Go version, connection/login booleans,
account count, and uptime—not account names, phone numbers, messages, or keys.
The configured intake key is redacted from authenticated report text before
sending. This is not a general-purpose secret scrubber: omit other secrets and
private chat data even though the intake also performs server-side redaction.

## Responses and retries

An authenticated POST must return HTTP `201` (created) or `200` (commented),
`ok: true`, the `whatsappgo` program, a recognized action, and an HTTP(S) item
URL before the app reports success. The URL may be a LAN address supplied by
Plane; opening it may require access to that network. Non-fatal intake warnings
do not turn an already-created report into a failed submission.

The client refuses redirects so neither credentials nor report bodies move to
another endpoint. Responses are bounded to 64 KiB and requests to 100 seconds.
There are no automatic POST retries. HTTP `400`, `401`, `413`, and `415` show
actionable errors; fix the input or configuration, or use the public form.
HTTP `429` enforces `Retry-After` (seconds or HTTP date) before another manual
submission; missing or invalid values use one minute. HTTP `502` means nothing
was filed and enforces a manual-retry backoff of 5 seconds, doubling to a maximum
of one minute. A confirmed success resets this backoff. A safe `X-Request-ID`
is included in failure feedback and daemon logs for operator troubleshooting;
report text, keys, arbitrary response bodies, and unsafe IDs are not logged.
A timeout or malformed success response may
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
service test checks destination disclosure, credential capability without secret
disclosure, and error propagation. Tests also cover `Retry-After`, manual-retry
backoff, terminal errors, known-key redaction, and request-ID safety. The
offscreen `desktop-bug-report` test checks that both desktop actions open the
exact GitHub issues URL without a dialog and show the URL on launch failure;
it stubs the opener and does not open a real browser or submit a report.

The maximum-history retention policy remains unchanged: disappearing-message
timers do not expire local conversation history. Explicit revocations and
deletion actions remain separate. See [the retention policy](ARCHITECTURE.md#local-history-retention-policy).
