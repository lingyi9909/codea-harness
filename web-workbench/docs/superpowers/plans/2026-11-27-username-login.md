# Username-only login

Goal: replace the shared name/phone form with one username field. Accept 1–80 ASCII letters/digits with at least one letter, reject spaces and punctuation, retain case sensitivity. Examples: zhang, zhang01.

Implementation:
- Validate the new form and the local/LAN API; stop submitting or saving a phone for username-only entry.
- Keep the existing browser credential and owner checks for LAN history, logout and peer isolation. Do not guess or merge old phone-based histories.
- Support the same form in HTTPS multi-user mode using a namespaced owner subject, without changing schema or inventing phone values. Preserve old API phone payloads for compatibility.
- Test invalid names, local login/logout, LAN history after logout/restart, distinct users, and the shared management-page login. Run API regressions, type checks, build and Chromium tests; add MySQL integration coverage for CI.

Changing from an old name/phone profile to username-only changes its owner key. Old histories are retained but are not automatically transferred. Local/LAN history recovery still needs the same browser and exact username.
