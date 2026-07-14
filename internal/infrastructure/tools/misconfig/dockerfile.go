package misconfig

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
)

var (
	secretKeyRe        = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|credential|\bauth\b)`)
	insecureDownloadRe = regexp.MustCompile(`(?i)(?:curl\b[^|&;]*\s(?:-k|--insecure)|wget\b[^|&;]*\s--no-check-certificate)`)
	aptCleanRe         = regexp.MustCompile(`(?i)rm\s+-rf?\s+[^\n]*/var/lib/apt/lists`)
	sudoRe             = regexp.MustCompile(`(?i)(?:^|[|&;]\s*|\s)sudo\s`)
	todoCommentRe      = regexp.MustCompile(`(?i)^\s*#\s*(?:todo|fixme|hack|xxx|bug)\b`)
)

// instruction is one logical Dockerfile instruction with its starting line (backslash continuations joined).
type instruction struct {
	cmd  string
	args string
	line int
}

// scanDockerfile runs the owned Dockerfile checks and returns located findings.
func scanDockerfile(rel string, data []byte) []ports.MisconfigRawFinding {
	instrs := parseDockerfile(string(data))
	out := checkDockerComments(string(data), rel)

	stageNames := map[string]bool{}
	var lastUserLine, lastFromLine int
	lastUserRoot := true
	haveStage, haveHealthcheck := false, false
	cmdCount, entrypointCount, healthcheckCount := 0, 0, 0

	for _, in := range instrs {
		switch in.cmd {
		case "FROM":
			haveStage, lastFromLine, lastUserLine, lastUserRoot = true, in.line, 0, true
			cmdCount, entrypointCount, healthcheckCount = 0, 0, 0
			img, alias := parseFrom(in.args)
			if r, ok := checkBaseImageTag(img, stageNames, rel, in.line); ok {
				out = append(out, r)
			}
			if r, ok := checkBaseImageDigest(img, stageNames, rel, in.line); ok {
				out = append(out, r)
			}
			if alias != "" {
				stageNames[strings.ToLower(alias)] = true
			}
		case "USER":
			lastUserLine, lastUserRoot = in.line, isRootUser(in.args)
		case "HEALTHCHECK":
			healthcheckCount++
			if healthcheckCount > 1 {
				out = append(out, dockerFinding(rel, in.line, "dockerfile-multiple-healthcheck", "Multiple HEALTHCHECK instructions", shared.SeverityLow, "Dockerfile HEALTHCHECK", "Only the last HEALTHCHECK instruction takes effect. Keep one health check per build stage."))
			}
			if !strings.EqualFold(strings.TrimSpace(in.args), "NONE") {
				haveHealthcheck = true
			}
		case "ENV", "ARG":
			if r, ok := checkSecretEnv(in.cmd, in.args, rel, in.line); ok {
				out = append(out, r)
			}
			if in.cmd == "ENV" && isLegacyEnv(in.args) {
				out = append(out, dockerFinding(rel, in.line, "dockerfile-env-legacy-format", "Legacy ENV key/value format", shared.SeverityInfo, "Dockerfile ENV", "Use ENV key=value syntax so values and subsequent environment assignments are unambiguous."))
			}
		case "ADD":
			if r, ok := checkAddRemote(in.args, rel, in.line); ok {
				out = append(out, r)
			} else if r, ok := checkAddLocal(in.args, rel, in.line); ok {
				out = append(out, r)
			}
		case "COPY":
			if isCopyRecursiveRoot(in.args) {
				out = append(out, dockerFinding(rel, in.line, "dockerfile-copy-recursive-root", "Build context copied into root filesystem", shared.SeverityLow, "Dockerfile COPY", "Copy application files into a dedicated directory instead of the root filesystem."))
			}
		case "RUN":
			if hasPipeToShell(in.args) {
				out = append(out, dockerFinding(rel, in.line, "dockerfile-run-pipe-shell", "Remote script piped to a shell", shared.SeverityHigh, "Dockerfile RUN", "A RUN step downloads a script and pipes it directly into a shell. Download to a file, verify a checksum or signature, then run it."))
			}
			out = append(out, checkRunStep(in.args, rel, in.line)...)
		case "EXPOSE":
			out = append(out, checkExpose(in.args, rel, in.line)...)
		case "WORKDIR":
			if isRelativeWorkdir(in.args) {
				out = append(out, dockerFinding(rel, in.line, "dockerfile-workdir-relative", "Relative WORKDIR path", shared.SeverityLow, "Dockerfile WORKDIR", "Use an absolute WORKDIR path so it does not depend on an earlier instruction."))
			}
		case "MAINTAINER":
			out = append(out, dockerFinding(rel, in.line, "dockerfile-maintainer-deprecated", "Deprecated MAINTAINER instruction", shared.SeverityInfo, "Dockerfile MAINTAINER", "Use an OCI image label such as org.opencontainers.image.authors instead."))
		case "CMD":
			cmdCount++
			if cmdCount > 1 {
				out = append(out, dockerFinding(rel, in.line, "dockerfile-multiple-cmd", "Multiple CMD instructions", shared.SeverityLow, "Dockerfile CMD", "Only the last CMD instruction takes effect. Combine the intended command into one instruction."))
			}
		case "ENTRYPOINT":
			entrypointCount++
			if entrypointCount > 1 {
				out = append(out, dockerFinding(rel, in.line, "dockerfile-multiple-entrypoint", "Multiple ENTRYPOINT instructions", shared.SeverityLow, "Dockerfile ENTRYPOINT", "Only the last ENTRYPOINT instruction takes effect. Keep one entrypoint per build stage."))
			}
			if !strings.HasPrefix(strings.TrimSpace(in.args), "[") {
				out = append(out, dockerFinding(rel, in.line, "dockerfile-entrypoint-shell-form", "Shell-form ENTRYPOINT", shared.SeverityLow, "Dockerfile ENTRYPOINT", "Use JSON exec form so the application receives signals directly."))
			}
		}
	}

	if haveStage && lastUserRoot {
		line := lastUserLine
		desc := "The final build stage sets USER root (or 0), so the container runs as root. Add a non-root USER as the last USER instruction."
		if lastUserLine == 0 {
			line = lastFromLine
			desc = "No USER instruction, so the container runs as root by default. Add a non-root USER before the entrypoint."
		}
		out = append(out, dockerFinding(rel, line, "dockerfile-run-as-root", "Container runs as root", shared.SeverityHigh, "Dockerfile USER", desc))
	}
	if haveStage && !haveHealthcheck {
		out = append(out, dockerFinding(rel, lastFromLine, "dockerfile-no-healthcheck", "No container HEALTHCHECK", shared.SeverityLow, "Dockerfile", "The image declares no HEALTHCHECK instruction, so an orchestrator cannot detect an unhealthy-but-running container. Add a HEALTHCHECK that probes the application's readiness."))
	}
	return out
}

func dockerFinding(file string, line int, id, title string, severity shared.Severity, resource, description string) ports.MisconfigRawFinding {
	return ports.MisconfigRawFinding{File: file, Line: line, RuleID: id, Title: title, Severity: severity, Resource: resource, Description: description}
}

// checkBaseImageTag flags a FROM that pins no immutable version: no tag, an explicit :latest, and no digest.
func checkBaseImageTag(img string, stageNames map[string]bool, rel string, line int) (ports.MisconfigRawFinding, bool) {
	if !isExternalImage(img, stageNames) || strings.Contains(img, "@sha256:") {
		return ports.MisconfigRawFinding{}, false
	}
	tag := imageTag(img)
	if tag != "" && tag != "latest" {
		return ports.MisconfigRawFinding{}, false
	}
	return dockerFinding(rel, line, "dockerfile-image-no-tag", "Base image is not version-pinned", shared.SeverityMedium, "Dockerfile FROM "+clip(img), "The base image uses no tag or :latest, so builds are not reproducible. Pin an explicit version tag, ideally with an @sha256 digest."), true
}

func checkBaseImageDigest(img string, stageNames map[string]bool, rel string, line int) (ports.MisconfigRawFinding, bool) {
	if !isExternalImage(img, stageNames) || strings.Contains(img, "@sha256:") {
		return ports.MisconfigRawFinding{}, false
	}
	return dockerFinding(rel, line, "dockerfile-image-no-digest", "Base image is not digest-pinned", shared.SeverityLow, "Dockerfile FROM "+clip(img), "A version tag can be moved or replaced. Pin the base image to an immutable sha256 digest for reproducible builds."), true
}

func isExternalImage(img string, stageNames map[string]bool) bool {
	return img != "" && img != "scratch" && !strings.Contains(img, "$") && !stageNames[strings.ToLower(img)]
}

func checkAddRemote(args, rel string, line int) (ports.MisconfigRawFinding, bool) {
	for _, f := range fields(args) {
		if strings.HasPrefix(f, "http://") || strings.HasPrefix(f, "https://") {
			return dockerFinding(rel, line, "dockerfile-add-remote-url", "ADD fetches a remote URL", shared.SeverityMedium, "Dockerfile ADD", "ADD with a remote URL downloads over the network with no integrity check. Use a RUN step that downloads and verifies a checksum, or COPY a vendored file."), true
		}
	}
	return ports.MisconfigRawFinding{}, false
}

func checkDockerComments(src, rel string) []ports.MisconfigRawFinding {
	var out []ports.MisconfigRawFinding
	for i, line := range strings.Split(src, "\n") {
		if todoCommentRe.MatchString(strings.TrimSuffix(line, "\r")) {
			out = append(out, dockerFinding(rel, i+1, "dockerfile-todo-comment", "TODO comment in Dockerfile", shared.SeverityInfo, "Dockerfile comment", "Resolve or track TODO-style comments outside the Dockerfile so the build definition stays maintainable."))
		}
	}
	return out
}

func parseDockerfile(src string) []instruction {
	lines := strings.Split(src, "\n")
	var out []instruction
	for i := 0; i < len(lines); {
		raw := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimSpace(raw)
		startLine := i + 1
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		full := trimmed
		for strings.HasSuffix(strings.TrimRight(full, " \t"), `\`) && i+1 < len(lines) {
			full = strings.TrimSuffix(strings.TrimRight(full, " \t"), `\`)
			i++
			next := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
			if strings.HasPrefix(next, "#") {
				full += ` \`
				continue
			}
			full += " " + next
		}
		full = strings.TrimSuffix(strings.TrimRight(full, " \t"), `\`)
		i++
		cmd, args := splitInstruction(full)
		if cmd != "" {
			out = append(out, instruction{cmd: strings.ToUpper(cmd), args: strings.TrimSpace(args), line: startLine})
		}
	}
	return out
}

func splitInstruction(line string) (cmd, args string) {
	line = strings.TrimSpace(line)
	sp := strings.IndexAny(line, " \t")
	if sp < 0 {
		return line, ""
	}
	return line[:sp], line[sp+1:]
}

func parseFrom(args string) (img, alias string) {
	var rest []string
	for _, tok := range fields(args) {
		if !strings.HasPrefix(tok, "--") {
			rest = append(rest, tok)
		}
	}
	if len(rest) == 0 {
		return "", ""
	}
	img = rest[0]
	for j := 1; j+1 < len(rest); j++ {
		if strings.EqualFold(rest[j], "AS") {
			return img, rest[j+1]
		}
	}
	return img, ""
}

func imageTag(img string) string {
	if i := strings.Index(img, "@"); i >= 0 {
		img = img[:i]
	}
	c := strings.LastIndex(img, ":")
	if c < 0 || strings.Contains(img[c:], "/") {
		return ""
	}
	return img[c+1:]
}

func isRootUser(args string) bool {
	u := strings.Fields(strings.TrimSpace(args))
	if len(u) == 0 {
		return true
	}
	user := strings.Split(u[0], ":")[0]
	return user == "root" || user == "0"
}

func fields(s string) []string { return strings.Fields(s) }

func checkSecretEnv(cmd, args, rel string, line int) (ports.MisconfigRawFinding, bool) {
	for _, kv := range envAssignments(args) {
		key, val := kv[0], strings.TrimSpace(kv[1])
		if secretKeyRe.MatchString(key) && val != "" && !strings.HasPrefix(val, "$") {
			return dockerFinding(rel, line, "dockerfile-secret-in-"+strings.ToLower(cmd), "Secret baked into image ("+cmd+")", shared.SeverityHigh, "Dockerfile "+cmd+" "+clip(key), "A secret-looking "+cmd+" key is assigned a literal value, so the credential is persisted in an image layer. Inject secrets at runtime or use BuildKit --secret."), true
		}
	}
	return ports.MisconfigRawFinding{}, false
}

func isLegacyEnv(args string) bool {
	f := fields(args)
	return len(f) >= 2 && !strings.Contains(f[0], "=")
}

func checkAddLocal(args, rel string, line int) (ports.MisconfigRawFinding, bool) {
	var src []string
	for _, tok := range fields(args) {
		if !strings.HasPrefix(tok, "--") {
			src = append(src, tok)
		}
	}
	if len(src) < 2 {
		return ports.MisconfigRawFinding{}, false
	}
	for _, s := range src[:len(src)-1] {
		if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || archivePath(s) {
			return ports.MisconfigRawFinding{}, false
		}
	}
	return dockerFinding(rel, line, "dockerfile-add-instead-of-copy", "ADD used for a local file (prefer COPY)", shared.SeverityLow, "Dockerfile ADD", "ADD copies a local file here, but it also fetches URLs and auto-extracts archives. Use COPY for local files so the intent is explicit."), true
}

func archivePath(path string) bool {
	low := strings.ToLower(path)
	for _, ext := range []string{".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".gz", ".xz", ".bz2"} {
		if strings.HasSuffix(low, ext) {
			return true
		}
	}
	return false
}

func isCopyRecursiveRoot(args string) bool {
	if strings.Contains(args, "--from") {
		return false
	}
	trimmed := strings.TrimSpace(args)
	if jsonStart := strings.IndexByte(trimmed, '['); jsonStart >= 0 && strings.HasSuffix(trimmed, "]") {
		f := strings.Split(strings.Trim(trimmed[jsonStart:], "[]"), ",")
		if len(f) != 2 {
			return false
		}
		return (strings.Trim(strings.TrimSpace(f[0]), `"`) == "." || strings.Trim(strings.TrimSpace(f[0]), `"`) == "./") && strings.Trim(strings.TrimSpace(f[1]), `"`) == "/"
	}
	var f []string
	for _, tok := range fields(args) {
		if !strings.HasPrefix(tok, "--") {
			f = append(f, tok)
		}
	}
	return len(f) == 2 && (f[0] == "." || f[0] == "./") && f[1] == "/"
}

func checkExpose(args, rel string, line int) []ports.MisconfigRawFinding {
	var out []ports.MisconfigRawFinding
	for _, token := range fields(args) {
		port := strings.Split(strings.ToLower(token), "/")[0]
		if port == "22" {
			out = append(out, dockerFinding(rel, line, "dockerfile-expose-ssh", "SSH port exposed", shared.SeverityMedium, "Dockerfile EXPOSE", "Exposing SSH expands the attack surface. Prefer application-specific ports and use controlled access for administration."))
		}
		if strings.ContainsAny(port, "$-") {
			continue
		}
		if n, err := strconv.Atoi(port); err == nil && (n < 1 || n > 65535) {
			out = append(out, dockerFinding(rel, line, "dockerfile-expose-invalid-port", "Invalid EXPOSE port", shared.SeverityLow, "Dockerfile EXPOSE", "EXPOSE declares a literal port outside the valid 1-65535 range."))
		}
	}
	return out
}

func isRelativeWorkdir(args string) bool {
	path := strings.TrimSpace(args)
	return path != "" && !strings.HasPrefix(path, "/") && !strings.Contains(path, "$")
}

func checkRunStep(args, rel string, line int) []ports.MisconfigRawFinding {
	var out []ports.MisconfigRawFinding
	if sudoRe.MatchString(args) {
		out = append(out, dockerFinding(rel, line, "dockerfile-run-sudo", "sudo used in a RUN step", shared.SeverityMedium, "Dockerfile RUN", "A RUN step uses sudo. Image builds already run as root, so sudo is unnecessary and expands the attack surface."))
	}
	if insecureDownloadRe.MatchString(args) {
		out = append(out, dockerFinding(rel, line, "dockerfile-insecure-download", "TLS verification disabled in a download", shared.SeverityMedium, "Dockerfile RUN", "A RUN step disables TLS verification during a download. Remove the flag and trust the system CA store."))
	}

	seen := map[string]bool{}
	appendUnique := func(findings []ports.MisconfigRawFinding) {
		for _, finding := range findings {
			if !seen[finding.RuleID] {
				seen[finding.RuleID] = true
				out = append(out, finding)
			}
		}
	}
	for _, command := range []string{"apt", "apt-get"} {
		for _, segment := range commandSegments(args, command, "install") {
			appendUnique(checkAptInstall(segment, command, rel, line))
		}
	}
	if hasUncleanAptInstall(args) {
		appendUnique([]ports.MisconfigRawFinding{dockerFinding(rel, line, "dockerfile-apt-no-clean", "apt install without cache cleanup", shared.SeverityLow, "Dockerfile RUN", "An apt install does not remove /var/lib/apt/lists afterward. Append rm -rf /var/lib/apt/lists/*.")})
	}
	for _, command := range []string{"apt", "apt-get"} {
		for _, subcommand := range []string{"upgrade", "dist-upgrade", "full-upgrade"} {
			if len(commandSegments(args, command, subcommand)) > 0 {
				appendUnique([]ports.MisconfigRawFinding{dockerFinding(rel, line, "dockerfile-apt-upgrade", "apt upgrade used in image build", shared.SeverityLow, "Dockerfile RUN", "Avoid upgrade commands in Dockerfiles; pin the base image and install only required packages.")})
			}
		}
	}
	for _, segment := range commandSegments(args, "apk", "add") {
		if !strings.Contains(segment, "--no-cache") {
			appendUnique([]ports.MisconfigRawFinding{dockerFinding(rel, line, "dockerfile-apk-no-cache", "apk add without --no-cache", shared.SeverityInfo, "Dockerfile RUN", "Use apk add --no-cache to avoid retaining the package index in the image.")})
		}
	}
	if hasUncleanRPMInstall(args) {
		appendUnique([]ports.MisconfigRawFinding{dockerFinding(rel, line, "dockerfile-rpm-no-clean", "RPM install without cache cleanup", shared.SeverityLow, "Dockerfile RUN", "Clean yum or dnf metadata in the same RUN instruction after installing packages.")})
	}
	if hasUnpinnedPackage(args, "pip", "pip") || hasUnpinnedPackage(args, "pip3", "pip") {
		appendUnique([]ports.MisconfigRawFinding{dockerFinding(rel, line, "dockerfile-pip-no-version-pin", "pip package is not version-pinned", shared.SeverityLow, "Dockerfile RUN", "Pin pip package versions to make builds reproducible.")})
	}
	return out
}

type runSegment struct {
	text string
	next string
}

func splitRunSegments(args string) []runSegment {
	var out []runSegment
	var current strings.Builder
	quote := byte(0)
	escaped := false
	flush := func(next string) {
		if text := strings.TrimSpace(current.String()); text != "" {
			out = append(out, runSegment{text: text, next: next})
		}
		current.Reset()
	}
	for i := 0; i < len(args); i++ {
		c := args[i]
		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			current.WriteByte(c)
			escaped = true
			continue
		}
		if quote != 0 {
			current.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			current.WriteByte(c)
			continue
		}
		if c == '&' && i > 0 && args[i-1] == '>' {
			current.WriteByte(c)
			continue
		}
		if c == '&' || c == ';' || c == '|' {
			next := string(c)
			if i+1 < len(args) && args[i+1] == c && c != ';' {
				next += string(c)
				i++
			}
			flush(next)
			continue
		}
		current.WriteByte(c)
	}
	flush("")
	return out
}

func commandSegments(args, command, subcommand string) []string {
	var out []string
	for _, segment := range splitRunSegments(args) {
		tokens := executableTokens(segment.text)
		if len(tokens) == 0 || !strings.EqualFold(tokens[0], command) {
			continue
		}
		for _, token := range tokens[1:] {
			if strings.EqualFold(token, subcommand) {
				out = append(out, segment.text)
				break
			}
		}
	}
	return out
}

func executableTokens(segment string) []string {
	tokens := fields(segment)
	for len(tokens) > 0 && (strings.HasPrefix(tokens[0], "--") || isShellAssignment(tokens[0])) {
		tokens = tokens[1:]
	}
	return tokens
}

func isShellAssignment(token string) bool {
	eq := strings.IndexByte(token, '=')
	if eq < 1 {
		return false
	}
	for i := 0; i < eq; i++ {
		c := token[i]
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func hasPipeToShell(args string) bool {
	segments := splitRunSegments(args)
	for i := 0; i+1 < len(segments); i++ {
		if segments[i].next != "|" {
			continue
		}
		left, right := executableTokens(segments[i].text), executableTokens(segments[i+1].text)
		if len(left) == 0 || len(right) == 0 {
			continue
		}
		left[0] = strings.Trim(left[0], "'\"")
		right[0] = strings.Trim(right[0], "'\"")
		if !strings.EqualFold(left[0], "curl") && !strings.EqualFold(left[0], "wget") {
			continue
		}
		if strings.EqualFold(right[0], "sudo") && len(right) > 1 {
			right = right[1:]
			right[0] = strings.Trim(right[0], "'\"")
		}
		if strings.EqualFold(right[0], "sh") || strings.EqualFold(right[0], "bash") {
			return true
		}
	}
	return false
}

func checkAptInstall(args, command, rel string, line int) []ports.MisconfigRawFinding {
	var out []ports.MisconfigRawFinding
	if !strings.Contains(args, "--no-install-recommends") {
		out = append(out, dockerFinding(rel, line, "dockerfile-apt-no-recommends", "apt install includes recommended packages", shared.SeverityInfo, "Dockerfile RUN", "Use --no-install-recommends unless recommended packages are intentionally required."))
	}
	if !hasAssumeYes(args) {
		out = append(out, dockerFinding(rel, line, "dockerfile-apt-no-assume-yes", "apt install lacks noninteractive confirmation", shared.SeverityLow, "Dockerfile RUN", "Use -y or --yes so image builds do not wait for interactive confirmation."))
	}
	if hasUnpinnedPackage(args, command, "apt") {
		out = append(out, dockerFinding(rel, line, "dockerfile-apt-no-version-pin", "apt package is not version-pinned", shared.SeverityLow, "Dockerfile RUN", "Pin apt package versions to make builds reproducible."))
	}
	return out
}

func hasUncleanAptInstall(args string) bool {
	needsCleanup := false
	previous := ""
	for _, segment := range splitRunSegments(args) {
		if len(commandSegments(segment.text, "apt", "install")) > 0 || len(commandSegments(segment.text, "apt-get", "install")) > 0 {
			needsCleanup = true
		}
		if needsCleanup && previous != "||" && previous != "|" && previous != "&" && aptCleanRe.MatchString(segment.text) {
			needsCleanup = false
		}
		previous = segment.next
	}
	return needsCleanup
}

func hasUncleanRPMInstall(args string) bool {
	needsCleanup := false
	previous := ""
	for _, segment := range splitRunSegments(args) {
		for _, command := range []string{"yum", "dnf"} {
			if len(commandSegments(segment.text, command, "install")) > 0 {
				needsCleanup = true
			}
			if previous != "||" && previous != "|" && previous != "&" && len(commandSegments(segment.text, command, "clean")) > 0 && strings.Contains(strings.ToLower(segment.text), "clean all") {
				needsCleanup = false
			}
		}
		previous = segment.next
	}
	return needsCleanup
}

func hasAssumeYes(args string) bool {
	return strings.Contains(args, "--yes") || strings.Contains(args, "--assume-yes") || regexp.MustCompile(`(?:^|\s)-[A-Za-z]*y[A-Za-z]*(?:\s|$)`).MatchString(args)
}

func hasUnpinnedPackage(args, command, manager string) bool {
	for _, segment := range commandSegments(args, command, "install") {
		tokens := fields(segment)
		install := -1
		for i, token := range tokens[1:] {
			if strings.EqualFold(token, "install") {
				install = i + 2
				break
			}
		}
		for i := install; i >= 0 && i < len(tokens); i++ {
			pkg := tokens[i]
			if redirect, needsTarget := isOutputRedirect(pkg); redirect {
				if needsTarget {
					i++
				}
				continue
			}
			if manager == "pip" && (pkg == "-e" || pkg == "--editable") {
				i++
				continue
			}
			if strings.HasPrefix(pkg, "-") || strings.Contains(pkg, "$") || pkg == "." || strings.HasPrefix(pkg, "/") || strings.HasPrefix(pkg, "./") || strings.HasPrefix(pkg, "http:") || strings.HasPrefix(pkg, "https:") || strings.HasSuffix(pkg, ".deb") {
				continue
			}
			if manager == "pip" && (strings.Contains(pkg, "requirements") || strings.HasPrefix(pkg, "--editable=")) {
				continue
			}
			if manager == "apt" && strings.Contains(pkg, "=") || manager == "pip" && (strings.Contains(pkg, "==") || strings.Contains(pkg, "@")) {
				continue
			}
			return true
		}
	}
	return false
}

func isOutputRedirect(token string) (bool, bool) {
	if !strings.HasPrefix(token, ">") && !regexp.MustCompile(`^\d*>>?`).MatchString(token) {
		return false, false
	}
	if token == ">" || token == ">>" || regexp.MustCompile(`^\d*>>?$`).MatchString(token) {
		return true, true
	}
	return true, false
}

// envAssignments parses modern K=V pairs and legacy ENV KEY value.
func envAssignments(args string) [][2]string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	if strings.Contains(args, "=") {
		var out [][2]string
		for _, tok := range splitEnvTokens(args) {
			if eq := strings.IndexByte(tok, '='); eq > 0 {
				out = append(out, [2]string{tok[:eq], strings.Trim(tok[eq+1:], `"'`)})
			}
		}
		return out
	}
	if f := strings.Fields(args); len(f) >= 2 {
		return [][2]string{{f[0], strings.Trim(strings.TrimSpace(args[len(f[0]):]), `"'`)}}
	} else if len(f) == 1 {
		return [][2]string{{f[0], ""}}
	}
	return nil
}

func splitEnvTokens(s string) []string {
	var toks []string
	var cur strings.Builder
	inQ := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQ != 0:
			if c == inQ {
				inQ = 0
			}
			cur.WriteByte(c)
		case c == '"' || c == '\'':
			inQ = c
			cur.WriteByte(c)
		case c == ' ' || c == '\t':
			if cur.Len() > 0 {
				toks = append(toks, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		toks = append(toks, cur.String())
	}
	return toks
}
