# Design notes

## Goals

- Run on a Linux endpoint with no dependencies and almost no footprint.
- Report configuration findings with the evidence behind them, so a result can be verified.
- Install and update like ordinary software (a Debian package), and run without supervision.
- Keep the cloud side serverless, so an idle system costs almost nothing and nothing needs patching.
- Define all infrastructure as code so it can be recreated or removed with one command.

## Agent

**Language and delivery.** Go, built with `CGO_ENABLED=0` into one static binary. It is packaged with `dpkg-deb` into a `.deb`, so it appears in `dpkg -l` and installs with `apt`. The package installs:

| Path | Purpose |
|---|---|
| `/usr/bin/go-agent` | the program |
| `/etc/go-agent/config.json` | API address and key, mode `0600`, listed as a conffile so upgrades keep local edits |
| `/lib/systemd/system/go-agent.service` | one-shot job that runs `go-agent send` |
| `/lib/systemd/system/go-agent.timer` | runs the job daily and five minutes after boot (`Persistent=true`) |

The package contains no secrets. The maintainer scripts only enable and disable the timer.

**Read-only.** The agent never modifies the system it inspects. When something it needs is missing (for example `/run/sshd` for `sshd -T`), it reports an `ERROR` with the reason instead of creating it.

**Four statuses.** `PASS` and `FAIL` are not enough. A check that cannot run must not be reported as either, and a component that is not installed is not a failure of the thing being checked. `NOT_APPLICABLE` and `ERROR` keep the report honest.

**Evidence.** Every result carries the value that was read (for example `ufw Status: inactive`). Where possible the agent reads effective values instead of parsing files: `sshd -T` for SSH, `modprobe -n -v` and `/proc/modules` for kernel modules, `systemctl` for services.

**Delivery to the cloud.** `go-agent send` posts the whole report to `/ingest`. Network errors and 5xx responses are retried up to three times with a growing delay. 4xx responses (a wrong key, a bad request) are not retried, since retrying cannot fix them. The process exits non-zero on failure so the scheduler records it. Settings are validated before the slow scan starts.

## Backend

```
API Gateway (REST, API key) ──► Lambda ingest ──► DynamoDB
                           └──► Lambda read   ──► DynamoDB
```

- **API Gateway REST API** rather than the HTTP API type, because REST APIs support API keys and usage plans directly. The usage plan throttles to 10 requests per second.
- **Two Lambdas** (Python 3.12). `ingest` handles `POST /ingest`; `read` handles the three `GET` routes. Table names are passed in as environment variables.
- **Least-privilege IAM.** `ingest` may only `PutItem` on the three tables; `read` may only `GetItem` and `Scan` on them. Both get CloudWatch logging.
- **CORS.** `OPTIONS` routes are answered by API Gateway without a key, because browsers send preflight requests without custom headers.

### Data model

Three tables, on-demand billing, each keyed by `machine_id` (the value of `/etc/machine-id`, unique per machine; hostnames are not).

| Table | One item per host holds |
|---|---|
| `hosts` | host details, `last_seen`, `package_count`, a status summary of the checks |
| `packages` | the installed package list |
| `cis_results` | the checks with status and evidence, and a summary |

One table per endpoint keeps each query trivial and keeps the large package list out of the host listing. A single-table design would also work and needs fewer writes. Each report overwrites the host's previous items, so the tables always show current state.

## Dashboard

Plain HTML, CSS and JavaScript with no build step. The API address comes from a `config.js` that Terraform generates, so no address is hard-coded. The API key is entered once and kept in the browser's local storage, never in the page source. A host's details are fetched the first time its row is opened. All API data is inserted with `textContent`: evidence strings originate on the scanned machines and must not be treated as markup.

Hosting is an S3 static website. CloudFront with a private bucket was the first choice, since it adds HTTPS, but AWS requires new accounts to be verified before creating CloudFront resources. Moving to CloudFront later only changes `infra/frontend.tf`.

## Checks: design notes

- **Unused filesystems.** `cramfs` and `squashfs` count as disabled when `modprobe -n -v` shows `install /bin/false` or the module is absent from the kernel, and they are not loaded. A module built into the kernel cannot be blocked this way and fails the check. Note that Ubuntu's `snap` depends on `squashfs`, so this control conflicts with a default Ubuntu feature and will normally fail there.
- **Time synchronisation.** Passes when either `chrony` or `systemd-timesyncd` is active.
- **World-writable files.** Runs `find / -xdev -type f -perm -0002` with a two-minute limit, so a very large disk cannot stall the agent.
- **Password complexity.** `minlen` ≥ 14 and either `minclass` ≥ 3 or all four `*credit` settings ≤ -1, read from `pwquality.conf` and its drop-ins. This is a simplified reading of the CIS controls and does not verify the PAM stack.

## Testing and environments

Developed in WSL (Ubuntu 24.04) and verified on Ubuntu 22.04.5 on EC2. WSL is not a representative server (no firewall, no auditd, AppArmor disabled), so many checks fail there for reasons unrelated to configuration; the EC2 results are the meaningful ones.
