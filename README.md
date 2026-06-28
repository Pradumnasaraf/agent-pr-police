[![build-ci][build-ci-badge]][build-ci] [![releases-ci][releases-ci-badge]][releases-ci] [![release][release-badge]][release] [![actions-marketplace][actions-marketplace-badge]][actions-marketplace]

## About (Agent PR Police)

**Agent PR Police** is a GitHub Action that adds security checks to pull requests opened by AI coding agents like GitHub Copilot, Claude Code, Devin, Cursor, and Codex. It works out whether a PR was authored by an agent, runs a set of security rules over the changed files, and leaves the findings in a single comment on the PR so you can review the risky parts before merging.

It runs on human PRs too. Agent PRs just get the extra checks.

## Features

- **Agent detection**: Detects agent-authored PRs from the author login, an `agent` label, or `Co-authored-by` commit trailers. You can add your own identifiers for agents that aren't built in.
- **Container security rules**: Flags unpinned base images, running as root, hardcoded secrets, `curl | sh`, `sudo`, `ADD` from a URL, and whole-context copies in Dockerfiles.
- **Sticky PR comment**: Posts one comment with the findings and updates it in place on every run, so you don't get duplicates.
- **SARIF report**: Writes a SARIF 2.1.0 file you can upload to GitHub code scanning.
- **Tunable gate**: Pick the severity that should fail the check (`none`, `low`, `medium`, `high`).

## Usage

The Action takes a few inputs, all of them optional with default values:

1. **`fail-on`** (Optional): Minimum severity that fails the check: `none`, `low`, `medium`, or `high`. Default is `high`.
2. **`comment-on-pr`** (Optional): Post and update a single sticky comment on the PR with the findings. Default is `true`.
3. **`label`** (Optional): Label name that marks a PR as agent-authored. Default is `agent`.
4. **`treat-all-prs-as-agent`** (Optional): Skip detection and run the policy on every PR. Default is `false`.
5. **`extra-agent-identifiers`** (Optional): Newline-separated substrings matched against the author login and co-author trailers, for agents not in the built-in list. Default is empty.
6. **`sarif-file`** (Optional): Path to write the SARIF report to. Default is `agent-pr-police.sarif`.
7. **`github-token`** (Optional): Token used to read the PR and post the comment. Default is `${{ github.token }}`.

The Action also sets these outputs: `is-agent-pr`, `findings` (JSON array), `high-count`, `medium-count`, and `low-count`.

### Event Trigger

Agent PR Police works on **pull request events**. It will not trigger on any other event.

```yaml
on:
  pull_request: # Works only on pull requests
```

### Permissions

To post the comment, the job needs write access to pull requests:

```yaml
permissions:
  contents: read
  pull-requests: write
```

### Example Workflow

> [!IMPORTANT]
> Before using the code below, check the latest `agent-pr-police` version in the `uses` field from the [GitHub Marketplace](https://github.com/marketplace/actions/agent-pr-police).

```yaml
name: Agent PR Police

on:
  pull_request: # Works only on pull requests

jobs:
  guard:
    runs-on: ubuntu-latest

    permissions:
      contents: read # Required for reading the PR
      pull-requests: write # Required for commenting on the PR

    steps:
      - name: Running Agent PR Police
        uses: Pradumnasaraf/agent-pr-police@v1
        with:
          fail-on: high # Optional
```

## Rules

Every finding comes with the file and line, why it matters, and a fix hint.

| Rule | Severity | What it catches |
| --- | --- | --- |
| `docker/unpinned-base-image` | High / Medium | `:latest` or no tag (High); a tag without a digest (Medium). |
| `docker/runs-as-root` | High | No `USER`, or an explicit `USER root` / UID `0` in the final stage. |
| `docker/hardcoded-secret` | High | A credential-like value baked into `ENV` / `ARG` / `RUN`. |
| `docker/remote-script-execution` | High | `curl` / `wget` piped into a shell. |
| `docker/sudo-usage` | Medium | `sudo` used inside the image build. |
| `docker/add-remote-url` | Medium | `ADD` with a remote URL instead of a verified download. |
| `docker/whole-context-copy` | Medium | `COPY . .` / `ADD . .` copying the whole build context. |

> Kubernetes, sensitive-path, and dependency rules are on the roadmap.

## Try it locally

The rule engine doesn't need GitHub, so you can scan files from your machine:

```bash
go run ./cmd/agent-pr-police scan examples/insecure.Dockerfile  # exits 1, prints findings
go run ./cmd/agent-pr-police scan examples/safe.Dockerfile      # exits 0, all clean
```

## Contributing

If you have suggestions for improving Agent PR Police or want to report a bug, feel free to open an issue! All contributions are welcome. For more details, check out the [Contributing Guide](CONTRIBUTING.md).

## License

This project is licensed under the [Apache License 2.0](LICENSE).

## Security

For information on reporting security vulnerabilities, please refer to the [Security Policy](SECURITY.md).

[build-ci]: https://github.com/Pradumnasaraf/agent-pr-police/actions/workflows/ci.yml
[build-ci-badge]: https://github.com/Pradumnasaraf/agent-pr-police/actions/workflows/ci.yml/badge.svg
[releases-ci]: https://github.com/Pradumnasaraf/agent-pr-police/actions/workflows/releases.yml
[releases-ci-badge]: https://github.com/Pradumnasaraf/agent-pr-police/actions/workflows/releases.yml/badge.svg
[release]: https://github.com/Pradumnasaraf/agent-pr-police/releases
[release-badge]: https://img.shields.io/github/v/release/Pradumnasaraf/agent-pr-police
[actions-marketplace]: https://github.com/marketplace/actions/agent-pr-police
[actions-marketplace-badge]: https://img.shields.io/badge/marketplace-Agent%20PR%20Police-blue?&logo=github
