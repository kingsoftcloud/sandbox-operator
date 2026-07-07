# Security Policy

## Supported Versions

The following versions of Sandbox Operator are currently supported with security updates:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in Sandbox Operator, please report it responsibly.

We ask that you:

* Do not open a public issue for the vulnerability.
* Provide a clear description of the issue and the steps to reproduce it, if possible.
* Include the affected component, version, and any potential impact.
* Allow reasonable time for us to investigate and release a fix before disclosing it publicly.

### How to report

Send an email to the repository maintainers with the subject line starting with `[SECURITY] Sandbox Operator`.

Please include:

* Your contact information.
* A detailed description of the vulnerability.
* Reproduction steps or a proof of concept.
* The version or commit you tested against.

We will acknowledge receipt as soon as possible and keep you informed of our progress.

## Disclosure Policy

When we receive a security report, we will:

1. Confirm the issue and determine its severity.
2. Develop and test a fix.
3. Release the fix in a timely manner.
4. Publish a security advisory describing the issue and the fix.
5. Credit the reporter unless they prefer to remain anonymous.

## Security Best Practices

When running Sandbox Operator in production, we recommend:

* Keep the operator image up to date.
* Restrict network access to the webhook service.
* Use cert-manager or properly managed TLS certificates for webhooks.
* Store OpenAPI credentials in Kubernetes Secrets with least-privilege RBAC.
* Regularly rotate OpenAPI access keys.
