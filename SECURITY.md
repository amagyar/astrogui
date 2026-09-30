# Security Policy

## Reporting a vulnerability

Please report security vulnerabilities privately through
[GitHub private vulnerability reporting](https://github.com/amagyar/astrogui/security/advisories/new).

Please do not open a public issue for anything you believe has security
impact.

## What to include

- the astrogui version (`astrogui version`)
- your operating system and how you installed astrogui (npm, Homebrew,
  or a self-built binary)
- a description of the issue and its impact, and steps or a proof of
  concept to reproduce it

## Expectations

- **Acknowledgement:** aim to acknowledge reports within 7 days.
- **Coordinated disclosure:** fixes are released before details are made
  public; we will agree on a publication timeline with you, credit you
  if you wish, and use a GitHub security advisory for the publication.

## Scope notes

astrogui is a local tool: it listens on loopback only, writes only
inside the managed draft and content directories plus its own config,
and never touches your project's manifests or configuration files.
Reports about that boundary (writes escaping the managed directories,
loopback exposure, unsafe handling of file contents or paths) are
especially welcome.
