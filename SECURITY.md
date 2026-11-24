# Security

ZOOMRIYA OS runs the ZOOMRIYA Box, which keeps a family's photos, messages and
accounts at home, so we treat security reports as the most important thing we
receive.

## Reporting a vulnerability

Report it privately through GitHub: **Security › Report a vulnerability** on
this repository. Please don't open a public issue, pull request or discussion
about it.

Include what you found, how to reproduce it, and what an attacker could do with
it. A proof of concept helps but isn't required.

## What happens next

- We acknowledge your report within 3 working days.
- We tell you whether we can reproduce it, and keep you updated until it's
  fixed.
- We agree a disclosure date with you, normally within 90 days of the report,
  sooner once a fix has shipped.
- We credit you in the advisory, unless you'd rather not be named.

## Scope

In scope: everything in this repository: the system image and how it's built,
`zoomriya-base`'s settings (firewall, AppArmor, updates, restore points),
`zoomriya-agent`, the first setup and Box Desktop on the box's screen, and the
USB installer.

A problem in one of Debian's own packages is best reported to the Debian
security team; tell us too if it affects the box, so we can get the fix to
families quickly.

Please don't test against other people's boxes or accounts, or access data
that isn't yours. Use your own box, or an image you build yourself
([README](README.md#building)).
