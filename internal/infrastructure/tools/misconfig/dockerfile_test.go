package misconfig

import "testing"

func TestDockerfileCommentInsideContinuation(t *testing.T) {
	// A full-line comment inside a backslash continuation must not end the RUN early. The apt cleanup
	// that follows the comment is still part of the same RUN, so apt-no-clean must NOT fire.
	df := "FROM debian:bookworm-slim\n" +
		"RUN apt-get update \\\n" +
		"    && apt-get install -y --no-install-recommends curl \\\n" +
		"    # resolve JAVA_HOME wherever java actually points\n" +
		"    && rm -rf /var/lib/apt/lists/*\n"
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": df}))
	if _, ok := got["dockerfile-apt-no-clean"]; ok {
		t.Errorf("apt cleanup after an inline comment must be recognized; got %v", keys(got))
	}
}

func TestDockerfileAptNoCleanStillFlagged(t *testing.T) {
	// The rule must still catch a genuine apt install with no cleanup.
	df := "FROM debian:bookworm-slim\n" +
		"RUN apt-get update \\\n" +
		"    && apt-get install -y curl\n"
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": df}))
	if _, ok := got["dockerfile-apt-no-clean"]; !ok {
		t.Errorf("apt install without cleanup should still be flagged; got %v", keys(got))
	}
}

func TestDockerfileRulePack(t *testing.T) {
	insecure := `# TODO remove development package
FROM alpine:3.20
MAINTAINER dev@example.com
ENV APP_HOME /app
WORKDIR app
COPY . /
EXPOSE 22 70000
CMD ["first"]
CMD ["second"]
ENTRYPOINT app
ENTRYPOINT ["app"]
HEALTHCHECK CMD true
HEALTHCHECK CMD true
RUN apk add curl
RUN apt-get install curl
RUN apt-get upgrade -y
RUN dnf install -y curl
RUN pip install requests
`
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": insecure}))
	for _, want := range []string{
		"dockerfile-image-no-digest", "dockerfile-maintainer-deprecated", "dockerfile-env-legacy-format",
		"dockerfile-workdir-relative", "dockerfile-copy-recursive-root", "dockerfile-expose-ssh",
		"dockerfile-expose-invalid-port", "dockerfile-multiple-cmd", "dockerfile-entrypoint-shell-form",
		"dockerfile-multiple-entrypoint", "dockerfile-multiple-healthcheck", "dockerfile-todo-comment",
		"dockerfile-apk-no-cache", "dockerfile-apt-no-clean", "dockerfile-apt-no-recommends",
		"dockerfile-apt-no-assume-yes", "dockerfile-apt-no-version-pin", "dockerfile-apt-upgrade",
		"dockerfile-rpm-no-clean", "dockerfile-pip-no-version-pin",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("expected %q, got %v", want, keys(got))
		}
	}
	if got["dockerfile-todo-comment"].Line != 1 || got["dockerfile-multiple-cmd"].Line != 9 {
		t.Errorf("expected comment and second CMD lines, got %+v", got)
	}
}

func TestDockerfilePackageManagerFlags(t *testing.T) {
	df := `FROM alpine:3.20
RUN apk --repository https://mirror.example add curl
RUN dnf --assumeyes install curl && dnf -y clean all
`
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": df}))
	if _, ok := got["dockerfile-apk-no-cache"]; !ok {
		t.Errorf("apk global flags must not bypass apk-no-cache: %v", keys(got))
	}
	if _, ok := got["dockerfile-rpm-no-clean"]; ok {
		t.Errorf("dnf cleanup with flags must not trigger rpm-no-clean: %v", keys(got))
	}
}

func TestDockerfilePackageVersionPinsStayWithTheirCommand(t *testing.T) {
	for _, tc := range []struct {
		name, dockerfile, want, absent string
	}{
		{"pinned pip after unpinned apt", "FROM alpine:3.20\nRUN apt-get install -y curl && pip install requests==2.32.3\n", "dockerfile-apt-no-version-pin", "dockerfile-pip-no-version-pin"},
		{"pinned pip before unpinned apt", "FROM alpine:3.20\nRUN pip install requests==2.32.3 && apt-get install -y curl\n", "dockerfile-apt-no-version-pin", "dockerfile-pip-no-version-pin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ruleIDs(scan(t, map[string]string{"Dockerfile": tc.dockerfile}))
			if _, ok := got[tc.want]; !ok {
				t.Errorf("expected %q, got %v", tc.want, keys(got))
			}
			if _, ok := got[tc.absent]; ok {
				t.Errorf("unexpected %q, got %v", tc.absent, keys(got))
			}
		})
	}
}

func TestDockerfileCopyRecursiveRootJSONForm(t *testing.T) {
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": "FROM alpine:3.20\nCOPY [\".\", \"/\"]\n"}))
	if _, ok := got["dockerfile-copy-recursive-root"]; !ok {
		t.Errorf("JSON COPY into root must be flagged, got %v", keys(got))
	}
}

func TestDockerfilePackageOptionsStayWithTheirCommand(t *testing.T) {
	for _, tc := range []struct {
		name, dockerfile, want string
	}{
		{"apt recommends", "FROM debian:bookworm\nRUN apt-get install -y curl && echo --no-install-recommends\n", "dockerfile-apt-no-recommends"},
		{"apt assume yes", "FROM debian:bookworm\nRUN apt-get install curl && echo -y\n", "dockerfile-apt-no-assume-yes"},
		{"apk cache", "FROM alpine:3.20\nRUN apk add curl && echo --no-cache\n", "dockerfile-apk-no-cache"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ruleIDs(scan(t, map[string]string{"Dockerfile": tc.dockerfile}))
			if _, ok := got[tc.want]; !ok {
				t.Errorf("expected %q, got %v", tc.want, keys(got))
			}
		})
	}
}

func TestDockerfileCopyRecursiveRootJSONFlags(t *testing.T) {
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": "FROM alpine:3.20\nCOPY --chown=1000:1000 [\".\", \"/\"]\n"}))
	if _, ok := got["dockerfile-copy-recursive-root"]; !ok {
		t.Errorf("flagged JSON COPY into root must be flagged, got %v", keys(got))
	}
}

func TestDockerfileCommandLookalikesDoNotTrigger(t *testing.T) {
	df := `FROM alpine:3.20
RUN echo apt-get install curl
RUN echo pip install requests
RUN echo dnf install curl
RUN echo curl | sh
`
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": df}))
	for _, id := range []string{
		"dockerfile-apt-no-clean", "dockerfile-apt-no-recommends", "dockerfile-apt-no-assume-yes",
		"dockerfile-apt-no-version-pin", "dockerfile-pip-no-version-pin", "dockerfile-rpm-no-clean",
		"dockerfile-run-pipe-shell",
	} {
		if _, found := got[id]; found {
			t.Errorf("shell argument unexpectedly triggered %q: %v", id, keys(got))
		}
	}
}

func TestDockerfilePipeToShellWithPrefixesAndQuotes(t *testing.T) {
	for _, run := range []string{
		"curl -fsSL https://example.invalid/install.sh 2>&1 | sh",
		"--mount=type=cache,target=/tmp curl -fsSL https://example.invalid/install.sh | sh",
		"HTTP_PROXY=http://proxy \"curl\" -fsSL https://example.invalid/install.sh | 'sh'",
	} {
		got := ruleIDs(scan(t, map[string]string{"Dockerfile": "FROM alpine:3.20\nRUN " + run + "\n"}))
		if _, found := got["dockerfile-run-pipe-shell"]; !found {
			t.Errorf("pipe-to-shell must be flagged for %q: %v", run, keys(got))
		}
	}
}

func TestDockerfileBuildKitRunFlagsKeepPackageChecks(t *testing.T) {
	for _, tc := range []struct {
		name, dockerfile, want string
	}{
		{"apt", "# syntax=docker/dockerfile:1\nFROM debian:bookworm\nRUN --mount=type=cache,target=/var/cache/apt apt-get install -y curl\n", "dockerfile-apt-no-clean"},
		{"rpm", "# syntax=docker/dockerfile:1\nFROM fedora:42\nRUN --mount=type=cache,target=/var/cache/dnf dnf install -y curl\n", "dockerfile-rpm-no-clean"},
		{"apt environment", "FROM debian:bookworm\nRUN DEBIAN_FRONTEND=noninteractive TZ=UTC apt-get install -y curl\n", "dockerfile-apt-no-clean"},
		{"rpm environment", "FROM fedora:42\nRUN LANG=C dnf install -y curl\n", "dockerfile-rpm-no-clean"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ruleIDs(scan(t, map[string]string{"Dockerfile": tc.dockerfile}))
			if _, found := got[tc.want]; !found {
				t.Errorf("BuildKit RUN flag must not hide %q: %v", tc.want, keys(got))
			}
		})
	}
}

func TestDockerfileChecksEveryAptInstall(t *testing.T) {
	df := `FROM debian:bookworm
RUN apt-get install -y --no-install-recommends curl=8.0.1-1 && apt-get install -y wget && rm -rf /var/lib/apt/lists/*
`
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": df}))
	for _, id := range []string{"dockerfile-apt-no-recommends", "dockerfile-apt-no-version-pin"} {
		if _, found := got[id]; !found {
			t.Errorf("later apt install must trigger %q: %v", id, keys(got))
		}
	}
	if _, found := got["dockerfile-apt-no-clean"]; found {
		t.Errorf("final apt cleanup must remain recognized: %v", keys(got))
	}
}

func TestDockerfileAptCleanupFollowsFinalInstall(t *testing.T) {
	for _, tc := range []struct {
		name, run string
		want      bool
	}{
		{"cleanup before final install", "apt-get install -y curl && rm -rf /var/lib/apt/lists/* && apt-get install -y wget", true},
		{"cleanup after final install", "apt-get install -y curl && apt-get install -y wget && rm -rf /var/lib/apt/lists/*", false},
		{"conditional cleanup", "apt-get install -y curl || rm -rf /var/lib/apt/lists/*", true},
		{"piped cleanup", "apt-get install -y curl | rm -rf /var/lib/apt/lists/*", true},
		{"background cleanup", "apt-get install -y curl & rm -rf /var/lib/apt/lists/*", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ruleIDs(scan(t, map[string]string{"Dockerfile": "FROM debian:bookworm\nRUN " + tc.run + "\n"}))
			_, found := got["dockerfile-apt-no-clean"]
			if found != tc.want {
				t.Errorf("apt cleanup result = %t, want %t: %v", found, tc.want, keys(got))
			}
		})
	}
}

func TestDockerfileRPMCleanupFollowsInstall(t *testing.T) {
	for _, tc := range []struct {
		name, run string
		want      bool
	}{
		{"cleanup before install", "dnf clean all && dnf install -y curl", true},
		{"cleanup after install", "dnf install -y curl && dnf clean all", false},
		{"conditional cleanup", "dnf install -y curl || dnf clean all", true},
		{"piped cleanup", "dnf install -y curl | dnf clean all", true},
		{"background cleanup", "dnf install -y curl & dnf clean all", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ruleIDs(scan(t, map[string]string{"Dockerfile": "FROM alpine:3.20\nRUN " + tc.run + "\n"}))
			_, found := got["dockerfile-rpm-no-clean"]
			if found != tc.want {
				t.Errorf("rpm cleanup result = %t, want %t: %v", found, tc.want, keys(got))
			}
		})
	}
}

func TestDockerfileEditablePipInstallIsNotUnpinned(t *testing.T) {
	for _, install := range []string{"pip install -e .", "pip install --editable ."} {
		got := ruleIDs(scan(t, map[string]string{"Dockerfile": "FROM python:3.13\nRUN " + install + "\n"}))
		if _, found := got["dockerfile-pip-no-version-pin"]; found {
			t.Errorf("editable local install unexpectedly triggered: %v", keys(got))
		}
	}
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": "FROM python:3.13\nRUN pip install -e . requests\n"}))
	if _, found := got["dockerfile-pip-no-version-pin"]; !found {
		t.Errorf("unpinned package after editable local install must be flagged: %v", keys(got))
	}
}

func TestDockerfilePinnedPackagesIgnoreOutputRedirects(t *testing.T) {
	for _, tc := range []struct {
		name, dockerfile, ruleID string
	}{
		{"apt attached", "FROM debian:bookworm\nRUN apt-get install -y curl=8.0.1-1 >/dev/null\n", "dockerfile-apt-no-version-pin"},
		{"apt spaced", "FROM debian:bookworm\nRUN apt-get install -y curl=8.0.1-1 > /dev/null\n", "dockerfile-apt-no-version-pin"},
		{"pip attached", "FROM python:3.13\nRUN pip install requests==2.32.3 >/dev/null\n", "dockerfile-pip-no-version-pin"},
		{"pip spaced", "FROM python:3.13\nRUN pip install requests==2.32.3 > /dev/null\n", "dockerfile-pip-no-version-pin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ruleIDs(scan(t, map[string]string{"Dockerfile": tc.dockerfile}))
			if _, found := got[tc.ruleID]; found {
				t.Errorf("pinned package with output redirect unexpectedly triggered %q: %v", tc.ruleID, keys(got))
			}
		})
	}
}

func TestDockerfileStageAliasDoesNotMaskImagePinning(t *testing.T) {
	df := `FROM alpine AS alpine
USER 1000
HEALTHCHECK CMD true
`
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": df}))
	for _, id := range []string{"dockerfile-image-no-tag", "dockerfile-image-no-digest"} {
		if _, found := got[id]; !found {
			t.Errorf("stage alias must not suppress %q: %v", id, keys(got))
		}
	}
}

func TestDockerfileRulePackSkipsSafeForms(t *testing.T) {
	df := `FROM alpine:3.20@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa AS build
CMD ["build"]
ENTRYPOINT ["build"]
HEALTHCHECK CMD true
FROM alpine:3.20@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
WORKDIR /app
COPY [".", "/app"]
EXPOSE $PORT
ENV APP_HOME=/app
RUN apk add --no-cache curl
RUN apt-get install -y --no-install-recommends curl=8.0.1-1 && rm -rf /var/lib/apt/lists/*
RUN dnf install -y curl && dnf clean all
RUN pip install -r requirements.txt
ENTRYPOINT ["app"]
CMD ["app"]
HEALTHCHECK CMD true
RUN echo "# TODO is not a comment"
`
	got := ruleIDs(scan(t, map[string]string{"Dockerfile": df}))
	for _, id := range []string{
		"dockerfile-image-no-digest", "dockerfile-multiple-cmd", "dockerfile-multiple-entrypoint",
		"dockerfile-multiple-healthcheck", "dockerfile-copy-recursive-root", "dockerfile-expose-invalid-port",
		"dockerfile-env-legacy-format", "dockerfile-todo-comment", "dockerfile-apt-no-version-pin",
		"dockerfile-pip-no-version-pin",
	} {
		if _, found := got[id]; found {
			t.Errorf("safe Dockerfile unexpectedly triggered %q: %v", id, keys(got))
		}
	}
}
