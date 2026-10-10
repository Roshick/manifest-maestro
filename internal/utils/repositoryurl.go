package utils

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const gitHubHost = "github.com"

var gitHubPathSegmentRegex = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// NormalizeGitHubRepositoryURL validates that repoURL points to a repository on github.com and
// returns its HTTPS form. Supported formats are:
//
//   - https://github.com/<owner>/<repo>[.git]
//   - git@github.com:<owner>/<repo>[.git]
//   - ssh://git@github.com[:22]/<owner>/<repo>[.git]
//
// Any other scheme, host, user, port, query or fragment is rejected, so that credentials are never
// sent to a host other than github.com.
func NormalizeGitHubRepositoryURL(repoURL string) (string, error) {
	invalid := func(reason string) (string, error) {
		return "", fmt.Errorf("repository url '%s' is invalid: %s", repoURL, reason)
	}

	path, ok := strings.CutPrefix(repoURL, "git@"+gitHubHost+":")
	if !ok {
		var reason string
		if path, reason = gitHubURLPath(repoURL); reason != "" {
			return invalid(reason)
		}
	}

	path = strings.TrimSuffix(path, "/")
	segments := strings.Split(path, "/")
	if len(segments) != 2 { //nolint:mnd // owner and repository
		return invalid("expected format <owner>/<repository>")
	}
	for _, segment := range segments {
		if !gitHubPathSegmentRegex.MatchString(segment) || strings.Trim(segment, ".") == "" {
			return invalid("owner or repository name contains invalid characters")
		}
	}
	if strings.TrimSuffix(segments[1], ".git") == "" {
		return invalid("repository name is missing")
	}

	return fmt.Sprintf("https://%s/%s/%s", gitHubHost, segments[0], segments[1]), nil
}

// gitHubURLPath returns the path of an https or ssh url pointing to github.com, or a reason why the
// url is not permitted.
func gitHubURLPath(repoURL string) (string, string) {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "cannot be parsed"
	}
	if !strings.EqualFold(parsed.Hostname(), gitHubHost) {
		return "", "only repositories on github.com are supported"
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", "query, fragment or opaque parts are not permitted"
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		if parsed.User != nil {
			return "", "user information is not permitted"
		}
		if parsed.Port() != "" && parsed.Port() != "443" {
			return "", "port is not permitted"
		}
	case "ssh":
		if parsed.User == nil || parsed.User.Username() != "git" {
			return "", "ssh urls must use the user 'git'"
		}
		if _, hasPassword := parsed.User.Password(); hasPassword {
			return "", "password is not permitted"
		}
		if parsed.Port() != "" && parsed.Port() != "22" {
			return "", "port is not permitted"
		}
	default:
		return "", "only https and ssh urls are supported"
	}
	return strings.TrimPrefix(parsed.Path, "/"), ""
}
