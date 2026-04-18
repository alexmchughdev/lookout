# Prompt — PDF test spec → lookout YAML

Paste everything below the `---` into Claude (or any capable LLM) along with your
test-spec PDF. It will emit a valid `lookout.yaml` you can drop straight into
`lookout run`.

Works best with **Claude Sonnet 4.6+**; Opus if the spec is long or ambiguous.
Claude Haiku also works for simple specs.

---

You are converting a QA test specification document into a `lookout.yaml` spec
for the **lookout** visual QA tool (https://github.com/alexmchughdev/lookout).

**How lookout works:** for each test, it navigates to a URL, optionally performs
a pre-action (click / type / drag), screenshots the page, and asks a local
vision model a yes/no question about the screenshot.

## Your task

Read the attached PDF. Extract every test case. Output a single YAML document
matching this schema exactly:

```yaml
app:
  url: https://REPLACE-ME.com
  auth:
    type: email_password        # or: session (for MFA/SSO apps)
    # email_field:    'input[name="username"]'
    # password_field: 'input[name="password"]'
    # submit_button:  'button[type="submit"]'
    # success_url_excludes: /login
    # login_path: /login        # override for non-/login URLs
    # For SSO/MFA sites, set type: session and run `lookout auth` first.

model:
  provider: ollama
  name: gemma3:12b

tests:
  - id: AUTH-01
    section: auth
    url: /login
    question: Is a login form with email and password fields visible?
    # optional fields shown in examples below
```

## Rules

1. **One test per YAML entry.** Do not merge multiple assertions into one test.
2. **`question` must be answerable from a single screenshot.**
   - Bad: "The user can submit the form and receive a confirmation email"
   - Good: "Is the Submit button visible and enabled below the form?"
   - Bad: "The search is fast"
   - Good: "Are search results visible within the results panel?"
3. **Questions are ONE sentence**, specific, unambiguous. No "and also" chains.
4. **`id`** format: `SEC-NN` (e.g. `AUTH-01`, `DASH-03`, `PIM-12`). 2–4-letter
   section prefix, zero-padded number. Reuse the prefix across related tests.
5. **`section`** = one lowercase word (`auth`, `dashboard`, `admin`, `nav`,
   `settings`, `billing`, `search`, `pim`, `reports`). Users filter with
   `--sections auth,dashboard` so keep these short and consistent.
6. **`url`** is the path only, starting with `/`. Never a full URL.
7. **Use `pre_action` when a test requires setup** before screenshotting:
   - `click` — `selector: 'text=My Note'`
   - `type_and_verify` — `click_selector`, `editor_selector`, `text` (types,
     saves, reloads, verifies the text survived)
   - `open_first` — `selector`, `fallback_button` (click the first item in
     a list, e.g. to open a workflow)
   - `drag` — `source`, `target`, `hold_ms`, `reload_after` (React DnD-safe)
   - `new_item` — `selector` pointing at a New/Create button
   - `select_option` — `selector` for the first option in a dropdown
   - `reload` — `wait_ms`
   - `wait` — `ms`
8. **SPAs that hydrate slowly** need `wait_for: '<CSS selector of a readiness
   marker>'` (preferred) or `wait_ms: 1500` (fallback).
9. **`full_page: false`** only if the test is specifically about what's
   above-the-fold. Default is full-page screenshots.
10. **Skip** sign-up flows, email/SMS verification, real payment flows, anything
    requiring a live third-party service. Flag these as a YAML comment:
    `# skipped: requires payment gateway`.
11. **If the PDF is ambiguous**, favour the simplest screenshot-answerable
    interpretation. Don't invent tests the PDF didn't describe.
12. **If the PDF mentions the target URL** (staging / prod), fill it into
    `app.url`. Otherwise leave `https://REPLACE-ME.com` as a placeholder.

## Worked example

If the PDF contains:

> **TC-AUTH-03** — After logging in, the user lands on the dashboard which
> displays a welcome header ("Welcome, <name>") and at least three stat tiles.

Output:

```yaml
  - id: AUTH-03
    section: dashboard
    url: /dashboard
    question: Is a welcome header visible alongside at least three stat tiles
      on the dashboard?
    wait_for: '[data-test="dashboard-ready"]'
```

If the PDF contains:

> **TC-NOTES-05** — When the user clicks an existing note in the sidebar, the
> editor loads that note's content.

Output:

```yaml
  - id: NOTES-05
    section: notes
    url: /notes
    question: Is a note editor visible with content rendered in the main panel?
    pre_action:
      type: click
      selector: 'aside [role="listitem"]:first-child'
```

## Output format

Output **only** the YAML document — no explanation, no markdown fences, no
preamble, no trailing commentary. The output should be directly writeable to
`lookout.yaml` with no editing.

If the PDF has N distinct test cases, you should produce N entries under
`tests:`. Preserve the PDF's test order.
