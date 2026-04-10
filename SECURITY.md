# Security Policy

## Supported versions

Only the latest minor version of sekd receives security fixes. Earlier versions should be upgraded before reporting.

| Version | Supported |
| ------- | --------- |
| 0.x     | ✅        |

## Reporting a vulnerability

If you believe you've found a security vulnerability in sekd — for example, a way to leak API keys from the config file, command injection via a ticker argument, or a path traversal in the cache — **please do not open a public issue**.

Instead:

1. Open a [private security advisory](https://github.com/jefrnc/sekd/security/advisories/new) on GitHub, or
2. Email the maintainer directly (see the commit history for the address).

Please include:

- A description of the vulnerability and its impact
- Steps to reproduce (a minimal test case if possible)
- The sekd version you tested against (`sekd version`)
- Any suggested fix, if you have one in mind

You will receive an acknowledgement within a few days. Valid reports will be fixed and disclosed publicly after a patch is released.

## Scope

In scope:

- The `sekd` binary itself
- Configuration and cache handling (`~/.sekd/config.json`, `~/.sekd/cache/`)
- Any code in this repository

Out of scope:

- Vulnerabilities in upstream dependencies — please report those to the relevant project directly
- Issues in SEC EDGAR, Finviz, OpenAI, or Anthropic services
- Social engineering or physical attacks

## Handling secrets

sekd stores API keys in `~/.sekd/config.json` with `0600` permissions. If you suspect a key has been leaked (for example, by pasting a config in a bug report), rotate it immediately at the provider's dashboard.
