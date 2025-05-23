package repos

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Arch struct {
	Idx      string   `json:"idx"`
	Location string   `json:"location"`
	Kmp      []string `json:"kmp"`
}

type Repo struct {
	Idx   string `json:"idx"`
	Amd64 Arch   `json:"amd64"`
}

type Distribution struct {
	Idx    string `json:"idx"`
	Drbd9  Repo   `json:"drbd-9"`
	Drbd90 Repo   `json:"drbd-9.0"`
}

type Repos struct {
	About struct {
		Created time.Time `json:"created"`
	} `json:"about"`
	Content struct {
		Rhel7  Distribution `json:"rhel7"`
		Rhel8  Distribution `json:"rhel8"`
		Rhel9  Distribution `json:"rhel9"`
		Rhel10 Distribution `json:"rhel10"`
	} `json:"content"`
}

func Get(ctx context.Context, pkgUrl string) (Repos, error) {
	var repoinfo Repos
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkgUrl, nil)
	if err != nil {
		return repoinfo, err
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return repoinfo, err
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&repoinfo); err != nil {
		return repoinfo, err
	}

	return repoinfo, nil
}

func (rs *Repos) GetKmps(repo, dist, arch string) []string {
	var d Distribution
	switch dist {
	case "rhel7":
		d = rs.Content.Rhel7
	case "rhel8":
		d = rs.Content.Rhel8
	case "rhel9":
		d = rs.Content.Rhel9
	case "rhel10":
		d = rs.Content.Rhel10
	default:
		return []string{}
	}

	var r Repo
	switch repo {
	case "drbd-9":
		r = d.Drbd9
	case "drbd-9.0":
		r = d.Drbd90
	default:
		return []string{}
	}

	switch arch {
	case "amd64":
		return r.Amd64.Kmp
	default:
		return []string{}
	}
}
