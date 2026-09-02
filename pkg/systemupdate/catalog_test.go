package systemupdate

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }
func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func testCatalog(client *http.Client, ttl time.Duration) *Catalog {
	return NewCatalog(CatalogConfig{ReleaseRepository: "VibeAPI/VibeAPI", ImageRepository: "heself/vibeapi", GitHubAPIBase: "https://catalog.test", DockerHubAPIBase: "https://catalog.test", CacheTTL: ttl}, client)
}

func TestCatalogJoinsReleaseHistoryWithImmutableDockerDigestAndCaches(t *testing.T) {
	githubRequests, dockerRequests := 0, 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/repos/VibeAPI/VibeAPI/releases":
			githubRequests++
			return jsonResponse(http.StatusOK, `[{"tag_name":"v4.10.15","name":"4.10.15","body":"fixed"},{"tag_name":"v4.10.14"},{"tag_name":"bad/tag"}]`), nil
		case "/v2/repositories/heself/vibeapi/tags":
			dockerRequests++
			return jsonResponse(http.StatusOK, `{"next":null,"results":[{"name":"v4.10.15","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","images":[{"architecture":"arm64","os":"linux"},{"architecture":"amd64","os":"linux"}]}]}`), nil
		default:
			return jsonResponse(http.StatusNotFound, "not found"), nil
		}
	})}
	catalog := testCatalog(client, time.Minute)
	first, err := catalog.List(t.Context(), "v4.10.14")
	require.NoError(t, err)
	require.Len(t, first.Releases, 2)
	assert.False(t, first.Cached)
	assert.Equal(t, "heself/vibeapi:v4.10.15", first.Releases[0].ImageRef)
	assert.Equal(t, "sha256:"+strings.Repeat("a", 64), first.Releases[0].Digest)
	assert.Equal(t, []string{"amd64", "arm64"}, first.Releases[0].Architectures)
	assert.True(t, first.Releases[0].Available)
	assert.True(t, first.Releases[1].Current)
	assert.False(t, first.Releases[1].Available)
	second, err := catalog.List(t.Context(), "v4.10.15")
	require.NoError(t, err)
	assert.True(t, second.Cached)
	assert.True(t, second.Releases[0].Current)
	assert.Equal(t, 1, githubRequests)
	assert.Equal(t, 1, dockerRequests)
}

func TestCatalogKeepsReleaseHistoryWhenRegistryIsUnavailable(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/repos/VibeAPI/VibeAPI/releases" {
			return jsonResponse(http.StatusOK, `[{"tag_name":"v1.0.0"}]`), nil
		}
		return jsonResponse(http.StatusServiceUnavailable, "temporary"), nil
	})}
	list, err := testCatalog(client, time.Minute).List(t.Context(), "")
	require.NoError(t, err)
	require.Len(t, list.Releases, 1)
	assert.False(t, list.Releases[0].Available)
	assert.Equal(t, "container registry is unavailable", list.Releases[0].UnavailableReason)
}

func TestCatalogReturnsExpiredCacheWhenGitHubBecomesUnavailable(t *testing.T) {
	fail := false
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if fail {
			return jsonResponse(http.StatusServiceUnavailable, "temporary"), nil
		}
		if r.URL.Path == "/repos/VibeAPI/VibeAPI/releases" {
			return jsonResponse(http.StatusOK, `[{"tag_name":"v1.0.0"}]`), nil
		}
		return jsonResponse(http.StatusOK, `{"next":null,"results":[{"name":"v1.0.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`), nil
	})}
	catalog := testCatalog(client, time.Nanosecond)
	first, err := catalog.List(t.Context(), "")
	require.NoError(t, err)
	require.Len(t, first.Releases, 1)
	fail = true
	time.Sleep(time.Millisecond)
	second, err := catalog.List(t.Context(), "")
	require.NoError(t, err)
	assert.True(t, second.Cached)
	require.Len(t, second.Releases, 1)
}

func TestCatalogFollowsSameOriginGitHubPagination(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/repos/VibeAPI/VibeAPI/releases" && r.URL.Query().Get("page") == "2" {
			return jsonResponse(http.StatusOK, `[{"tag_name":"v1.0.0"}]`), nil
		}
		if r.URL.Path == "/repos/VibeAPI/VibeAPI/releases" {
			response := jsonResponse(http.StatusOK, `[{"tag_name":"v2.0.0"}]`)
			response.Header.Set("Link", `<https://catalog.test/repos/VibeAPI/VibeAPI/releases?per_page=100&page=2>; rel="next"`)
			return response, nil
		}
		return jsonResponse(http.StatusOK, `{"next":null,"results":[]}`), nil
	})}
	list, err := testCatalog(client, time.Minute).List(t.Context(), "")
	require.NoError(t, err)
	require.Len(t, list.Releases, 2)
	assert.Equal(t, "v2.0.0", list.Releases[0].ID)
	assert.Equal(t, "v1.0.0", list.Releases[1].ID)
}
