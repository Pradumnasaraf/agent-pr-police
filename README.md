# Agent PR Police

**Agent PR Police** is a GitHub Action that flags pull requests opened by AI coding agents like GitHub Copilot, Claude Code, Devin, Cursor, and Codex. When it detects an agent PR, it adds a label and posts a single comment summarizing which agent opened it and what the PR touched, so reviewers know at a glance that a bot wrote the change.

It does not gate or block anything. It is a transparency layer, not a security scanner.

![Agent PR Police comment on a pull request](https://github.com/user-attachments/assets/e09f376e-1d24-41a6-bc27-d19505073872)

## Features

- **Agent detection**: Detects agent-authored PRs from the author login, the branch name, `Co-authored-by` commit trailers, markers in the PR description, or a label. Add your own identifiers for agents that aren't built in.
- **Auto label**: Applies a `pr-by-ai` label (configurable) to detected agent PRs. If you add the label by hand, the PR is treated as agent-authored too.
- **Sticky summary comment**: Posts one comment with the agent name and a summary of what the PR changed (files, added and removed lines, top areas). It updates in place on every run, so no duplicates.
- **Notify reviewers**: Optionally cc a user or team in the comment and request their review on agent PRs. The PR author is never pinged.
- **Never blocks**: It only informs, it never gates a PR. Permission and API errors are logged as warnings, so the check never fails.
- **Reusable outputs**: Later steps in your workflow can read `is-agent-pr` (`true`/`false`) and `agent` (the detected name) to add your own automation, like routing reviewers or gating elsewhere.
- **Opt in or out**: The label, comment, and reviewer notifications are each independently toggleable.

## Detected agents

GitHub Copilot, Claude Code, Devin, Cursor, OpenAI Codex, Aider, Google Jules, Sourcegraph Amp, Sweep, Amazon Q, OpenHands, Charlie, Ellipsis, Factory, Tembo, Zencoder, Codegen, and v0.

Each agent is matched on the signals that fit it: a distinctive login for agents that open PRs under a bot account, a branch prefix (like `copilot/` or `cursor/`), a `Co-authored-by` trailer, or a marker in the PR description (like Claude Code's "Generated with Claude Code" footer). Agents whose tooling commits under a human account (like Claude Code and Aider) are matched by trailer or PR body rather than login, to avoid false positives on human names.

## Usage

All inputs are optional.

| Input | Default | Description |
| --- | --- | --- |
| `label` | `pr-by-ai` | Label applied to detected agent PRs. Also recognized as an agent marker if added manually. |
| `add-label` | `true` | Apply the label to detected agent PRs. |
| `comment` | `true` | Post and update a single sticky comment summarizing the PR. |
| `treat-all-prs-as-agent` | `false` | Skip detection and treat every PR as agent-authored. |
| `extra-agent-identifiers` | `` | Newline-separated substrings matched against the author login, branch name, and co-author trailers, for agents not in the built-in registry. |
| `mention` | `` | Handles to `cc` in the comment on agent PRs, a single user or team, or several (e.g. `@alice`, or `@alice @org/team`). The PR author is skipped. Needs `comment` enabled. |
| `request-reviewers` | `` | Users and teams to request a review from on agent PRs, a single one or several (e.g. `@alice`, or `@alice @org/team`). The PR author is skipped. Failures (no access, etc.) are ignored, never fatal. |
| `github-token` | `${{ github.token }}` | Token used to read the PR, add the label, and post the comment. |

Outputs: `is-agent-pr` (`true` or `false`) and `agent` (the detected agent name, empty if none).

### Event trigger

Agent PR Police runs on **pull request events** only, on **Linux runners** (it ships as a Docker container action).

```yaml
on:
  pull_request:
```

### Permissions

To add the label and post the comment, the job needs write access to pull requests:

```yaml
permissions:
  contents: read
  pull-requests: write
```

### Minimal workflow

The shortest setup, every input uses its default:

```yaml
name: Agent PR Police

on:
  pull_request:

jobs:
  police:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - name: Running Agent PR Police
        uses: Pradumnasaraf/agent-pr-police@v1
```

### Full example with every option

Every input is optional. The values below are the defaults, so this behaves the same as the minimal workflow above; change only what you need.

> [!IMPORTANT]
> Before using the snippet below, check the latest version in the `uses` field from the [GitHub Marketplace](https://github.com/marketplace/actions/agent-pr-police).

```yaml
name: Agent PR Police

on:
  pull_request:

jobs:
  police:
    runs-on: ubuntu-latest
    permissions:
      contents: read # Required to read the PR
      pull-requests: write # Required to add the label and post the comment
    steps:
      - name: Running Agent PR Police
        uses: Pradumnasaraf/agent-pr-police@v1
        with:
          label: pr-by-ai # Optional. Label applied to agent PRs
          add-label: true # Optional. Apply the label
          comment: true # Optional. Post the summary comment
          treat-all-prs-as-agent: false # Optional. Treat every PR as agent
          extra-agent-identifiers: | # Optional. Extra agent matches, one per line
            acme-ai
            my-internal-bot
          mention: "@octocat @my-org/reviewers" # Optional. cc these users/teams in the comment
          request-reviewers: "@octocat @my-org/reviewers" # Optional. request review from these users/teams
          github-token: ${{ github.token }} # Optional. Defaults to GITHUB_TOKEN
```

Both jobs also expose outputs you can use in later steps: `is-agent-pr` (`true` or `false`) and `agent` (the detected agent name, empty if none).

## Contributing

If you have suggestions for improving Agent PR Police or want to report a bug, feel free to open an issue. All contributions are welcome. For more details, check out the [Contributing Guide](CONTRIBUTING.md).

## License

This project is licensed under the [Apache License 2.0](LICENSE).

## Security

For information on reporting security vulnerabilities, please refer to the [Security Policy](SECURITY.md).
