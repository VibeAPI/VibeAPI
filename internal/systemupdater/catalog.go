package systemupdater

import (
	"context"
	"fmt"
	"strings"

	contract "github.com/QuantumNous/new-api/pkg/systemupdate"
)

type Catalog interface {
	List(context.Context) (contract.ReleaseList, error)
	Get(context.Context, string) (contract.Release, error)
}

type ReleaseCatalog struct {
	catalog    *contract.Catalog
	repository string
}

func NewReleaseCatalog(catalog *contract.Catalog, repository string) *ReleaseCatalog {
	return &ReleaseCatalog{catalog: catalog, repository: repository}
}

func (c *ReleaseCatalog) List(ctx context.Context) (contract.ReleaseList, error) {
	list, err := c.catalog.List(ctx, "")
	if err != nil {
		return contract.ReleaseList{}, err
	}
	seen := make(map[string]struct{}, len(list.Releases))
	for i := range list.Releases {
		release := &list.Releases[i]
		if !contract.ValidReleaseID(release.ID) {
			return contract.ReleaseList{}, fmt.Errorf("catalog contains invalid release id")
		}
		if _, ok := seen[release.ID]; ok {
			return contract.ReleaseList{}, fmt.Errorf("catalog contains duplicate release id %q", release.ID)
		}
		seen[release.ID] = struct{}{}
		if repositoryFromImageRef(release.ImageRef) != c.repository {
			return contract.ReleaseList{}, fmt.Errorf("release %q is outside allowed image repository", release.ID)
		}
		if release.Available && !contract.ValidDigest(release.Digest) {
			return contract.ReleaseList{}, fmt.Errorf("release %q has invalid digest", release.ID)
		}
	}
	return list, nil
}

func (c *ReleaseCatalog) Get(ctx context.Context, releaseID string) (contract.Release, error) {
	list, err := c.List(ctx)
	if err != nil {
		return contract.Release{}, err
	}
	for _, release := range list.Releases {
		if release.ID == releaseID {
			return release, nil
		}
	}
	return contract.Release{}, fmt.Errorf("release %q not found", releaseID)
}

func repositoryFromImageRef(ref string) string {
	if at := strings.IndexByte(ref, '@'); at >= 0 {
		return ref[:at]
	}
	lastSlash := strings.LastIndexByte(ref, '/')
	lastColon := strings.LastIndexByte(ref, ':')
	if lastColon > lastSlash {
		return ref[:lastColon]
	}
	return ref
}
