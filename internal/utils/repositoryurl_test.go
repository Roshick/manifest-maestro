package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeGitHubRepositoryURL_Valid(t *testing.T) {
	cases := map[string]string{
		"https://github.com/org/repo":            "https://github.com/org/repo",
		"https://github.com/org/repo.git":        "https://github.com/org/repo.git",
		"https://GitHub.com/org/repo/":           "https://github.com/org/repo",
		"https://github.com:443/org/repo.git":    "https://github.com/org/repo.git",
		"git@github.com:org/repo.git":            "https://github.com/org/repo.git",
		"git@github.com:_/service-a.git":         "https://github.com/_/service-a.git",
		"ssh://git@github.com/org/repo.git":      "https://github.com/org/repo.git",
		"ssh://git@github.com:22/org/my.repo":    "https://github.com/org/my.repo",
		"ssh://git@github.com/org/repo_name.git": "https://github.com/org/repo_name.git",
	}
	for in, expected := range cases {
		actual, err := NormalizeGitHubRepositoryURL(in)
		require.NoError(t, err, in)
		require.Equal(t, expected, actual, in)
	}
}

func TestNormalizeGitHubRepositoryURL_Invalid(t *testing.T) {
	cases := []string{
		"",
		"http://github.com/org/repo.git",
		"https://attacker.example/org/repo.git",
		"https://github.com.attacker.example/org/repo.git",
		"https://github.com@attacker.example/org/repo.git",
		"https://user:pass@github.com/org/repo.git",
		"https://github.com:8443/org/repo.git",
		"https://github.com/org/repo.git?x=1",
		"https://github.com/org/repo.git#x",
		"https://github.com/org",
		"https://github.com/org/repo/extra",
		"https://github.com/../repo",
		"https://github.com/org/..",
		"https://github.com/org/.git",
		"ssh://github.com/org/repo.git",
		"ssh://root@github.com/org/repo.git",
		"ssh://git:secret@github.com/org/repo.git",
		"ssh://git@attacker.example/org/repo.git",
		"git@attacker.example:org/repo.git",
		"git@github.com:org/repo/extra.git",
		"file:///etc/passwd",
		"/tmp/repo.git",
		"github.com/org/repo",
	}
	for _, in := range cases {
		_, err := NormalizeGitHubRepositoryURL(in)
		require.Error(t, err, in)
	}
}
