package rules

import (
	"path"
	"regexp"
	"strings"
)

// DockerfileRule implements the container-security checks (group A).
type DockerfileRule struct{}

// NewDockerfileRule constructs the Dockerfile rule.
func NewDockerfileRule() *DockerfileRule { return &DockerfileRule{} }

// ID identifies the rule family.
func (r *DockerfileRule) ID() string { return "docker" }

// instruction is a single parsed Dockerfile directive.
type instruction struct {
	cmd  string // upper-cased keyword, e.g. "FROM"
	args string // everything after the keyword (continuations joined)
	line int    // 1-based line where the instruction begins
}

// IsDockerfile reports whether a path looks like a Dockerfile. It matches a
// bare "Dockerfile", the "Dockerfile.<suffix>" convention (e.g. Dockerfile.prod)
// and the "<name>.dockerfile" / "<name>.Dockerfile" convention.
func IsDockerfile(p string) bool {
	base := path.Base(p)
	lower := strings.ToLower(base)
	switch {
	case lower == "dockerfile":
		return true
	case strings.HasPrefix(lower, "dockerfile."):
		return true
	case strings.HasSuffix(lower, ".dockerfile"):
		return true
	default:
		return false
	}
}

// Check runs every Dockerfile check against the file.
func (r *DockerfileRule) Check(file ChangedFile) []Finding {
	if !IsDockerfile(file.Path) {
		return nil
	}
	instrs := parseDockerfile(file.Content)
	var findings []Finding

	findings = append(findings, r.checkBaseImages(file.Path, instrs)...)
	findings = append(findings, r.checkUser(file.Path, instrs)...)
	for _, in := range instrs {
		findings = append(findings, r.checkInstruction(file.Path, in)...)
	}
	return findings
}

// parseDockerfile turns raw content into instructions, joining line
// continuations and skipping comments and blank lines.
func parseDockerfile(content string) []instruction {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var instrs []instruction
	i := 0
	for i < len(lines) {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		startLine := i + 1
		i++
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Join continuation lines (ending in backslash).
		combined := strings.TrimSuffix(trimmed, "\\")
		for strings.HasSuffix(strings.TrimSpace(trimmed), "\\") && i < len(lines) {
			next := strings.TrimSpace(lines[i])
			i++
			trimmed = next
			combined += " " + strings.TrimSuffix(next, "\\")
			if !strings.HasSuffix(next, "\\") {
				break
			}
		}
		fields := strings.Fields(combined)
		if len(fields) == 0 {
			continue
		}
		cmd := strings.ToUpper(fields[0])
		args := strings.TrimSpace(strings.TrimPrefix(combined, fields[0]))
		instrs = append(instrs, instruction{cmd: cmd, args: args, line: startLine})
	}
	return instrs
}

var (
	// curl/wget piped to a shell.
	reRemoteExec = regexp.MustCompile(`(?i)\b(curl|wget)\b[^|]*\|\s*(sudo\s+)?(sh|bash|zsh)\b`)
	// ADD with a remote URL.
	reRemoteURL = regexp.MustCompile(`^(https?|git|ftp)://`)
	// Sensitive key assignment with a literal value.
	reSecret = regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:password|passwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|credential)[A-Z0-9_]*)\s*[=:]\s*(\S+)`)
	// A value that is just a variable reference (not a hardcoded secret).
	reVarRef = regexp.MustCompile(`^["']?\$[\{(]?[A-Za-z_]`)
)

// checkBaseImages flags unpinned FROM base images. It tracks stage aliases so
// that "FROM builder" references to earlier stages are not flagged.
func (r *DockerfileRule) checkBaseImages(file string, instrs []instruction) []Finding {
	var findings []Finding
	stages := map[string]bool{}
	for _, in := range instrs {
		if in.cmd != "FROM" {
			continue
		}
		fields := strings.Fields(in.args)
		if len(fields) == 0 {
			continue
		}
		image := fields[0]
		// Record stage alias: FROM <image> AS <name>.
		if len(fields) >= 3 && strings.EqualFold(fields[1], "AS") {
			stages[strings.ToLower(fields[2])] = true
		}
		// Skip references to a previously defined stage and scratch.
		if stages[strings.ToLower(image)] || strings.EqualFold(image, "scratch") {
			continue
		}
		// Pinned by digest is always acceptable.
		if strings.Contains(image, "@sha256:") {
			continue
		}
		name, tag := splitImageTag(image)
		_ = name
		switch {
		case tag == "latest":
			findings = append(findings, Finding{
				RuleID:   "docker/unpinned-base-image",
				Severity: SeverityHigh,
				File:     file,
				Line:     in.line,
				Title:    "Base image uses the :latest tag",
				Message:  "The :latest tag is mutable, so builds are not reproducible and a compromised or changed upstream image is pulled silently.",
				FixHint:  "Pin to a specific version and ideally a digest, e.g. FROM " + name + ":1.2.3@sha256:<digest>.",
			})
		case tag == "":
			findings = append(findings, Finding{
				RuleID:   "docker/unpinned-base-image",
				Severity: SeverityHigh,
				File:     file,
				Line:     in.line,
				Title:    "Base image has no tag (implicitly :latest)",
				Message:  "An untagged image resolves to :latest, which is mutable and not reproducible.",
				FixHint:  "Add an explicit version tag and digest, e.g. FROM " + name + ":1.2.3@sha256:<digest>.",
			})
		default:
			findings = append(findings, Finding{
				RuleID:   "docker/unpinned-base-image",
				Severity: SeverityMedium,
				File:     file,
				Line:     in.line,
				Title:    "Base image is not pinned by digest",
				Message:  "A version tag can be re-pointed to different content. Pinning by digest guarantees the exact image bytes.",
				FixHint:  "Append a digest, e.g. FROM " + image + "@sha256:<digest>.",
			})
		}
	}
	return findings
}

// splitImageTag splits an image reference into name and tag, ignoring any
// digest. A registry host with a port (host:5000/img) is handled by only
// treating the final path segment's colon as a tag separator.
func splitImageTag(image string) (name, tag string) {
	ref := image
	if at := strings.Index(ref, "@"); at >= 0 {
		ref = ref[:at]
	}
	slash := strings.LastIndex(ref, "/")
	lastSeg := ref
	prefix := ""
	if slash >= 0 {
		prefix = ref[:slash+1]
		lastSeg = ref[slash+1:]
	}
	if colon := strings.LastIndex(lastSeg, ":"); colon >= 0 {
		return prefix + lastSeg[:colon], lastSeg[colon+1:]
	}
	return ref, ""
}

// checkUser flags running as root in the final build stage.
func (r *DockerfileRule) checkUser(file string, instrs []instruction) []Finding {
	lastFromLine := 0
	currentUser := ""
	currentUserLine := 0
	for _, in := range instrs {
		switch in.cmd {
		case "FROM":
			lastFromLine = in.line
			// New stage resets the effective user.
			currentUser = ""
			currentUserLine = 0
		case "USER":
			currentUser = strings.ToLower(strings.TrimSpace(in.args))
			currentUserLine = in.line
		}
	}
	if lastFromLine == 0 {
		return nil // not a real Dockerfile
	}
	if currentUser == "" {
		return []Finding{{
			RuleID:   "docker/runs-as-root",
			Severity: SeverityHigh,
			File:     file,
			Line:     lastFromLine,
			Title:    "Container runs as root (no USER directive)",
			Message:  "Without a USER directive the container runs as root, so a process escape has host-root-equivalent privileges inside the container.",
			FixHint:  "Create and switch to an unprivileged user, e.g. RUN adduser -D app && USER app.",
		}}
	}
	if currentUser == "root" || currentUser == "0" || strings.HasPrefix(currentUser, "root:") || strings.HasPrefix(currentUser, "0:") {
		return []Finding{{
			RuleID:   "docker/runs-as-root",
			Severity: SeverityHigh,
			File:     file,
			Line:     currentUserLine,
			Title:    "Container explicitly runs as root",
			Message:  "USER root (or UID 0) gives the running process full privileges in the container, widening the blast radius of any compromise.",
			FixHint:  "Switch to a non-root user before the final CMD/ENTRYPOINT.",
		}}
	}
	return nil
}

// checkInstruction runs the per-instruction checks.
func (r *DockerfileRule) checkInstruction(file string, in instruction) []Finding {
	var findings []Finding

	// Hardcoded secrets in ENV/ARG/RUN.
	if in.cmd == "ENV" || in.cmd == "ARG" || in.cmd == "RUN" {
		if m := reSecret.FindStringSubmatch(in.args); m != nil {
			value := m[2]
			if !reVarRef.MatchString(value) && value != "" {
				findings = append(findings, Finding{
					RuleID:   "docker/hardcoded-secret",
					Severity: SeverityHigh,
					File:     file,
					Line:     in.line,
					Title:    "Possible hardcoded secret",
					Message:  "A credential-like value (" + m[1] + ") is baked into the image. Anyone who can pull the image can read it from the layer history.",
					FixHint:  "Pass secrets at runtime (env, mounted file, or BuildKit --secret); never bake them into ENV/ARG/RUN.",
				})
			}
		}
	}

	if in.cmd == "RUN" {
		if reRemoteExec.MatchString(in.args) {
			findings = append(findings, Finding{
				RuleID:   "docker/remote-script-execution",
				Severity: SeverityHigh,
				File:     file,
				Line:     in.line,
				Title:    "Remote script piped directly into a shell",
				Message:  "Piping curl/wget output straight into sh/bash executes unverified remote code at build time, a classic supply-chain risk.",
				FixHint:  "Download to a file, verify a checksum or signature, then execute it.",
			})
		}
		if reSudo.MatchString(in.args) {
			findings = append(findings, Finding{
				RuleID:   "docker/sudo-usage",
				Severity: SeverityMedium,
				File:     file,
				Line:     in.line,
				Title:    "Use of sudo inside the image",
				Message:  "sudo is unnecessary in a container build and can mask privilege assumptions or enable escalation.",
				FixHint:  "Run the build steps as the appropriate user directly; drop sudo.",
			})
		}
	}

	// ADD with a remote URL.
	if in.cmd == "ADD" {
		fields := strings.Fields(in.args)
		for _, f := range fields {
			if reRemoteURL.MatchString(f) {
				findings = append(findings, Finding{
					RuleID:   "docker/add-remote-url",
					Severity: SeverityMedium,
					File:     file,
					Line:     in.line,
					Title:    "ADD used with a remote URL",
					Message:  "ADD with a URL fetches remote content without checksum verification and is easy to get subtly wrong.",
					FixHint:  "Use COPY for local files, or RUN curl with a verified checksum for remote downloads.",
				})
				break
			}
		}
	}

	// Whole-context copy: COPY . . / ADD . .
	if in.cmd == "COPY" || in.cmd == "ADD" {
		if isWholeContextCopy(in.args) {
			findings = append(findings, Finding{
				RuleID:   "docker/whole-context-copy",
				Severity: SeverityMedium,
				File:     file,
				Line:     in.line,
				Title:    "Whole build context copied into the image",
				Message:  "Copying the entire context risks leaking secrets, .git history, and local config into the image and bloats it.",
				FixHint:  "Copy only what is needed, and add a .dockerignore for sensitive/large paths.",
			})
		}
	}

	return findings
}

var reSudo = regexp.MustCompile(`(?i)\bsudo\b`)

// isWholeContextCopy reports whether a COPY/ADD copies the entire context.
func isWholeContextCopy(args string) bool {
	// Strip --flag options like --chown=...
	var fields []string
	for _, f := range strings.Fields(args) {
		if strings.HasPrefix(f, "--") {
			continue
		}
		fields = append(fields, f)
	}
	if len(fields) != 2 {
		return false
	}
	src := strings.Trim(fields[0], `"`)
	return src == "." || src == "./"
}
