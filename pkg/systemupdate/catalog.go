package systemupdate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type CatalogConfig struct {
	ReleaseRepository string
	ImageRepository   string
	GitHubAPIBase     string
	DockerHubAPIBase  string
	GitHubToken       string
	CacheTTL          time.Duration
	RequestTimeout    time.Duration
}

type Catalog struct {
	config    CatalogConfig
	client    *http.Client
	now       func() time.Time
	mu        sync.Mutex
	cached    []Release
	expiresAt time.Time
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
}

type dockerTagPage struct {
	Next    string `json:"next"`
	Results []struct {
		Name   string `json:"name"`
		Digest string `json:"digest"`
		Images []struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"images"`
	} `json:"results"`
}

func NewCatalog(config CatalogConfig, client *http.Client) *Catalog {
	if config.GitHubAPIBase == "" {
		config.GitHubAPIBase = "https://api.github.com"
	}
	if config.DockerHubAPIBase == "" {
		config.DockerHubAPIBase = "https://hub.docker.com"
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 5 * time.Minute
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 10 * time.Second
	}
	if client == nil {
		client = &http.Client{Timeout: config.RequestTimeout}
	}
	return &Catalog{config: config, client: client, now: time.Now}
}

func (catalog *Catalog) List(ctx context.Context, currentVersion string) (ReleaseList, error) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	now := catalog.now()
	if len(catalog.cached) > 0 && now.Before(catalog.expiresAt) {
		return ReleaseList{Releases: markCurrent(catalog.cached, currentVersion), Cached: true}, nil
	}

	releases, err := catalog.fetchGitHubReleases(ctx)
	if err != nil {
		if len(catalog.cached) > 0 {
			return ReleaseList{Releases: markCurrent(catalog.cached, currentVersion), Cached: true}, nil
		}
		return ReleaseList{}, err
	}
	tags, err := catalog.fetchDockerTags(ctx)
	if err != nil {
		for i := range releases {
			releases[i].Available = false
			releases[i].UnavailableReason = "container registry is unavailable"
		}
		catalog.cached = releases
		catalog.expiresAt = now.Add(catalog.config.CacheTTL)
		return ReleaseList{Releases: markCurrent(releases, currentVersion)}, nil
	}
	for i := range releases {
		if tag, ok := tags[releases[i].ID]; ok {
			releases[i].Digest = tag.digest
			releases[i].Architectures = tag.architectures
			releases[i].Available = tag.digest != ""
			if !releases[i].Available {
				releases[i].UnavailableReason = "docker image digest is unavailable"
			}
		} else {
			releases[i].UnavailableReason = "docker image tag is unavailable"
		}
	}
	catalog.cached = releases
	catalog.expiresAt = now.Add(catalog.config.CacheTTL)
	return ReleaseList{Releases: markCurrent(releases, currentVersion)}, nil
}

func (catalog *Catalog) fetchGitHubReleases(ctx context.Context) ([]Release, error) {
	repository := strings.Trim(catalog.config.ReleaseRepository, "/")
	if strings.Count(repository, "/") != 1 {
		return nil, errors.New("release repository must be owner/name")
	}
	endpoint := strings.TrimRight(catalog.config.GitHubAPIBase, "/") + "/repos/" + repository + "/releases?per_page=100"
	releases := make([]Release, 0)
	for endpoint != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "vibeapi-system-update")
		if catalog.config.GitHubToken != "" {
			req.Header.Set("Authorization", "Bearer "+catalog.config.GitHubToken)
		}
		resp, err := catalog.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch releases: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("fetch releases: unexpected status %d", resp.StatusCode)
		}
		var payload []githubRelease
		err = common.DecodeJson(resp.Body, &payload)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode releases: %w", err)
		}
		for _, item := range payload {
			if item.Draft || !ValidReleaseID(item.TagName) {
				continue
			}
			releases = append(releases, Release{
				ID: item.TagName, Version: item.TagName, Name: item.Name,
				PublishedAt: item.PublishedAt, ReleaseNotes: item.Body, ReleaseURL: item.HTMLURL,
				ImageRef: catalog.config.ImageRepository + ":" + item.TagName,
			})
		}
		next := parseNextLink(resp.Header.Get("Link"))
		if next != "" && !sameOrigin(next, catalog.config.GitHubAPIBase) {
			return nil, errors.New("release pagination returned an invalid next URL")
		}
		endpoint = next
	}
	return releases, nil
}

func parseNextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		sections := strings.Split(strings.TrimSpace(part), ";")
		if len(sections) < 2 || !strings.Contains(strings.Join(sections[1:], ";"), `rel="next"`) {
			continue
		}
		return strings.Trim(strings.TrimSpace(sections[0]), "<>")
	}
	return ""
}

func sameOrigin(raw, base string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return false
	}
	return parsed.Scheme == baseURL.Scheme && parsed.Host == baseURL.Host
}

type dockerTagInfo struct {
	digest        string
	architectures []string
}

func (catalog *Catalog) fetchDockerTags(ctx context.Context) (map[string]dockerTagInfo, error) {
	repository := strings.Trim(catalog.config.ImageRepository, "/")
	if strings.Count(repository, "/") != 1 {
		return nil, errors.New("image repository must be namespace/name")
	}
	endpoint := strings.TrimRight(catalog.config.DockerHubAPIBase, "/") + "/v2/repositories/" + repository + "/tags?page_size=100"
	result := map[string]dockerTagInfo{}
	for endpoint != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		resp, err := catalog.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch docker tags: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("fetch docker tags: unexpected status %d", resp.StatusCode)
		}
		var page dockerTagPage
		err = common.DecodeJson(resp.Body, &page)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode docker tags: %w", err)
		}
		for _, tag := range page.Results {
			architectures := make([]string, 0, len(tag.Images))
			seen := map[string]struct{}{}
			for _, image := range tag.Images {
				if image.OS != "linux" || image.Architecture == "" {
					continue
				}
				if _, ok := seen[image.Architecture]; ok {
					continue
				}
				seen[image.Architecture] = struct{}{}
				architectures = append(architectures, image.Architecture)
			}
			sort.Strings(architectures)
			result[tag.Name] = dockerTagInfo{digest: tag.Digest, architectures: architectures}
		}
		if page.Next == "" {
			break
		}
		if !sameOrigin(page.Next, catalog.config.DockerHubAPIBase) {
			return nil, errors.New("docker tag pagination returned an invalid next URL")
		}
		endpoint = page.Next
	}
	return result, nil
}

func markCurrent(releases []Release, currentVersion string) []Release {
	copyOfReleases := append([]Release(nil), releases...)
	for i := range copyOfReleases {
		copyOfReleases[i].Current = versionsEquivalent(copyOfReleases[i].Version, currentVersion)
	}
	return copyOfReleases
}

func versionsEquivalent(left, right string) bool {
	left = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(left), "v"), "V")
	right = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(right), "v"), "V")
	return left != "" && left == right
}
