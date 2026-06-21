# Security Policy

## Supported Versions

Inventario is currently in **private alpha** and does not yet publish versioned
releases with independent maintenance windows. Security fixes land on the
`master` branch, and the only supported version is the **latest `master`**.
Please reproduce any issue against the latest `master` before reporting.

| Version          | Supported          |
| ---------------- | ------------------ |
| latest `master`  | :white_check_mark: |
| older commits    | :x:                |

## Reporting a Vulnerability

**Please do not open a public GitHub issue, pull request, or discussion for
security vulnerabilities.** Public disclosure before a fix is available puts
alpha testers — some of whom store real inventory data — at risk.

Use one of these **private** channels instead:

1. **GitHub Private Vulnerability Reporting (preferred).** Go to the
   repository's **Security** tab and choose **Report a vulnerability**
   (<https://github.com/denisvmedia/inventario/security/advisories/new>).
   This keeps the report private and lets us coordinate a fix and advisory
   directly through GitHub.
2. **Email (fallback).** If you cannot use GitHub's private reporting, email
   the maintainer at **ask+security@stokaro.com**. Use a clear subject line such as
   `[SECURITY] Inventario vulnerability report`.

When reporting, please include as much detail as you can:

- A description of the vulnerability and its potential impact.
- Steps to reproduce (a proof of concept is ideal).
- The affected component, endpoint, or file, and the `master` commit you
  tested against.
- Any suggested remediation, if you have one.

## Response and Disclosure

- We aim to **acknowledge** your report within **3 business days**.
- We aim to provide an initial **assessment** (severity, whether we can
  reproduce, and a rough remediation plan) within **7 business days**.
- We follow a **coordinated disclosure** model: we will work with you on a fix
  and a disclosure timeline, and we ask that you keep the details private until
  a fix is available and affected users have had a reasonable window to update.
- With your permission, we are happy to credit you in the resulting advisory.

Thank you for helping keep Inventario and its users safe.
