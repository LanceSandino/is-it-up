# Security policy

## Supported versions

Security fixes are applied to the latest released version.

Credential-bearing database, Redis, and RabbitMQ checks use verified TLS by
default. Plaintext or identity-skipping compatibility must be selected
explicitly in configuration. HTTP checks do not follow redirects, and resolved
secret values are redacted from diagnostics and configuration output.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting feature for this
repository. Do not open a public issue with exploit details, credentials,
private endpoints, or other sensitive data.

Include the affected version, impact, reproduction steps, and any suggested
mitigation. You should receive an acknowledgement within seven days.
