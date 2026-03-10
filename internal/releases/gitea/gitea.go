// Package gitea fetches releases from a Gitea (or Forgejo) instance.
//
// Gitea's release API is GitHub-compatible but lives under /api/v1:
//
//	GET {baseurl}/api/v1/repos/{owner}/{repo}/releases
//
// This package appends the /api/v1 prefix and delegates to [githubish].
package gitea

import (
	"context"
	"iter"
	"net/http"
	"strings"

	"github.com/webinstall/webi-installers/internal/releases/githubish"
)

// Fetch retrieves releases from a Gitea instance.
// The baseURL should be the Gitea root (e.g. "https://git.rootprojects.org"),
// not the API path — /api/v1 is appended automatically.
func Fetch(ctx context.Context, client *http.Client, baseURL, owner, repo string, auth *githubish.Auth) iter.Seq2[[]githubish.Release, error] {
	return githubish.Fetch(ctx, client, strings.TrimRight(baseURL, "/")+"/api/v1", owner, repo, auth)
}
