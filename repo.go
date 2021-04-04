package main

import (
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

type repos struct {
	About struct {
		Created time.Time `json:"created"`
	} `json:"about"`
	Content struct {
		Rhel7 Distribution `json:"rhel7"`
		Rhel8 Distribution `json:"rhel8"`
	} `json:"content"`
}

func getRepos(pkgUrl string) (repos, error) {
	var repoinfo repos
	r, err := http.Get(pkgUrl)
	if err != nil {
		return repoinfo, err
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&repoinfo); err != nil {
		return repoinfo, err
	}

	return repoinfo, nil
}

func (rs *repos) getKmps(repo, dist, arch string) []string {
	var d Distribution
	switch dist {
	case "rhel7":
		d = rs.Content.Rhel7
	case "rhel8":
		d = rs.Content.Rhel8
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
