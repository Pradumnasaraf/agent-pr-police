> IMPORTANT **Note**
>
> **Pull Requests having no issue associated with them will not be accepted. Firstly get an issue assigned, whether it's already opened or raised by you, and then create a Pull Request.**

## 👨‍💻 Prerequisite

#### Documentation

- [Git](https://git-scm.com/)
- [Markdown](https://www.markdownguide.org/basic-syntax/)

#### Code

- [Go](https://go.dev/) (see the version in [`go.mod`](go.mod))
- [GitHub Actions](https://docs.github.com/en/actions)

## 💥 How to Contribute

- Look at the existing [**Issues**](https://github.com/Pradumnasaraf/agent-pr-police/issues) or [**create a new issue**](https://github.com/Pradumnasaraf/agent-pr-police/issues/new/choose)!
- [**Fork the Repo**](https://github.com/Pradumnasaraf/agent-pr-police/fork). Then, create a branch for any issue that you are working on. Finally, commit your work.
- Create a **[Pull Request](https://github.com/Pradumnasaraf/agent-pr-police/compare)** (_PR_), which will be promptly reviewed and given suggestions for improvements by the community.
- Add screenshots or screen captures to your Pull Request to help us understand the effects of the changes proposed in your PR.

## 🧪 Local Development

This is a [composite GitHub Action](action.yml) backed by a small Go program. The rule core (`internal/rules`) has no GitHub dependencies, so you can run and test everything locally.

```bash
# Build the binary
go build ./...

# Run the full test suite
go test ./...

# Check formatting and run static analysis
gofmt -l .
go vet ./...

# Scan local files without any GitHub calls (great for trying out rules)
go run ./cmd/agent-pr-police scan examples/insecure.Dockerfile
go run ./cmd/agent-pr-police scan examples/safe.Dockerfile
```

Please make sure `go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt` all pass before opening a PR.

### Adding a new rule

1. Implement the `rules.Rule` interface in a new file under `internal/rules`.
2. Wire it into `DefaultRules` in `internal/rules/rule.go`.
3. Add table-driven tests alongside it.

### Adding a new agent to the detection registry

Append an entry to `Registry` in `internal/detect/detect.go` with distinctive login and/or co-author trailer patterns, and add a test case.

If you need any assistance or have further questions, feel free to ask. Happy contributing!
