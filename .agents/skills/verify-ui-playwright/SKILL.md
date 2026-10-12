---
name: verify-ui-playwright
description: Verify user-facing changes in uptime-app after implementation using Playwright MCP. Check affected workflows, responsive layout, browser console, and network results; reproduce and fix defects within the task scope. Use before reporting UI or browser-visible behavior changes as complete, or when explicitly asked to verify them.
---

# Verify UI with Playwright

Use the running application and Playwright MCP to establish whether the implemented
change meets the task's expected behavior and layout. Browser checks supplement
required project checks; they do not replace them.

## Establish the scope and environment

- Read the task requirements, relevant diff, and affected project's guide. Derive
  observable acceptance checks from the requested behavior, design reference, or
  existing UI conventions. Do not treat the implementation itself as the specification.
- Identify affected routes, shared components, user states, and critical adjacent
  flows. For a shared component, inspect representative affected pages rather than
  assuming that one page covers every layout.
- Use [the root README](../../../README.md) for startup and canonical origins and
  [the frontend guide](../../../frontend/README.md) for UI conventions and checks.
  Reuse the existing development environment; start it with the documented command
  if needed. Avoid competing servers on the same ports.
- Use Playwright MCP for browser work. If the tools or application are unavailable,
  diagnose the blocker and report the checks that remain unverified. Do not silently
  substitute mocked responses or a different browser runner for this verification.
- Inspect the current browser session before changing users. Prefer existing test
  data; create uniquely named local test data only when the scenario needs it.
  Keep mutations within the authorized task and test environment. Do not remove
  existing user data or run destructive scenarios against it.

## Verify behavior through the UI

- Navigate to the affected route. Use `browser_snapshot` or `browser_find` to locate
  controls, and use the MCP click, fill, select, and keyboard tools to interact.
  Refresh snapshots after transitions so element references remain current.
  Use `browser_evaluate` for focused read-only observations such as element geometry
  or image loading attributes; do not bypass the UI with API calls or state injection.
- Exercise the requested happy path from the user's entry point to the final result.
  Check visible feedback and resulting state, not just whether a click succeeded.
- Select relevant edge cases based on the change: invalid input, empty states,
  cancellation, pending controls, validation recovery, or authentication boundaries.
  Inspect loading/error states when they are reachable; do not trigger every possible
  state for a minor change or manufacture failures through unrelated configuration.
- For data mutations, verify the corresponding request outcome and read the result
  again after a refresh or revisit. For local preferences, verify persistence only
  when required by the task. Never repeat a mutation after a timeout until its actual
  outcome is established.
- Check keyboard reachability, accessible control names, and visible focus for
  changed interactive elements. For affected dialogs, verify opening, dismissal,
  initial focus, and focus return.

## Inspect layout visually

- Inspect the affected page at desktop and mobile sizes with `browser_resize`.
  Use representative viewports such as 1280 x 900 and 390 x 844; adjust them to the
  task's target devices and inspect a nearby breakpoint if behavior changes there.
- Capture screenshots with `browser_take_screenshot` and actually view the returned
  image or saved file. Accessibility snapshots and DOM measurements alone cannot
  establish visual correctness. Use snapshots to target interactions, not screenshot
  coordinates. Wait for relevant content, fonts, and images before capturing.
- Compare against supplied designs or the existing project UI. Inspect alignment,
  spacing, typography, wrapping, clipping, overlap, contrast, and horizontal overflow.
  Scroll to inspect content below the fold and check fixed elements or dialogs when
  affected. Check long content only where it could change the layout.
- Save relevant screenshots and diagnostic files under the ignored
  `.playwright-mcp/` directory with descriptive names. The development launcher clears
  that directory: preserve needed evidence before restarting it. Do not commit
  browser artifacts. Attach appropriate screenshots if the task includes a UI PR.

## Inspect console and network evidence

- Use `browser_console_messages` for messages from the verification navigation and
  `browser_network_requests` for relevant requests. Record a baseline when needed to
  distinguish existing issues from regressions; do not attribute old messages to the
  current change without reproduction.
- Inspect specific response bodies/statuses when needed to confirm a result. Avoid
  exporting cookies, authorization headers, passwords, or full sensitive payloads.
- Investigate new console errors, warnings, failed resources, and unexpected API
  responses. A cancelled request may be intentional: correlate it with navigation
  or effect cleanup and the final successful result before classifying it.
- A successful status alone does not prove functional correctness, and a warning
  alone does not prove failure. Correlate browser evidence with the expected result.

## Resolve findings and report

- Reproduce defects with a concrete route, viewport, user state, and action. Fix
  defects within the authorized implementation scope, then rerun the affected
  scenario and required project checks appropriate to the fix. Report unrelated
  findings without expanding the task to fix them.
- Restore temporary viewport settings and clean up only test data created for this
  verification when appropriate. Preserve data the user requested as a deliverable.
- Report which workflows and viewports were checked, whether they passed, any
  remaining defects or blocked checks, and relevant artifact paths. Distinguish
  observed results from inferences. Never claim completion for an untested critical
  acceptance check or imply that browser verification proves full test coverage.
