package main

import (
	"fmt"
	"regexp"
	"strconv"
)

type version struct {
	major, minor, patch int
	pkg                 string
}

type versionMap map[string]version

func (v *version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

func (v *version) isLargerThan(w version) bool {
	if v.major > w.major {
		return true
	} else if w.major > v.major {
		return false
	}

	if v.minor > w.minor {
		return true
	} else if w.minor > v.minor {
		return false
	}

	if v.patch > w.patch {
		return true
	} else if w.patch > v.patch {
		return false
	}

	return false
}

// if they are the same return w
func versionMax(v, w version) version {
	if v.isLargerThan(w) {
		return v
	}
	return w
}

func getMajorMinorPatch(major, minor, patch string) (int, int, int, error) {
	maj, min, pat := 0, 0, 0
	var err error

	maj, err = strconv.Atoi(major)
	if err != nil {
		return maj, min, pat, err
	}
	min, err = strconv.Atoi(minor)
	if err != nil {
		return maj, min, pat, err
	}
	pat, err = strconv.Atoi(patch)
	if err != nil {
		return maj, min, pat, err
	}

	return maj, min, pat, err
}

func filterKmps(kmps []string) (versionMap, error) {
	have := make(versionMap)

	re := regexp.MustCompile(`^kmod-drbd-(\d+)\.(\d+)\.(\d+)_(.*)-.*\.rpm$`)
	for _, file := range kmps {
		matches := re.FindStringSubmatch(file)
		if len(matches) != 5 {
			continue
		}
		major, minor, patch, err := getMajorMinorPatch(matches[1], matches[2], matches[3])
		if err != nil {
			continue
		}
		v := version{
			major: major,
			minor: minor,
			patch: patch,
			pkg:   matches[0],
		}
		kpart := matches[4]

		ev, ok := have[kpart]
		if ok {
			have[kpart] = versionMax(ev, v)
		} else {
			have[kpart] = v
		}
	}

	vmax := version{
		major: 0,
		minor: 0,
		patch: 0,
	}
	for _, v := range have {
		vmax = versionMax(vmax, v)
	}
	for k, v := range have {
		if vmax.isLargerThan(v) {
			delete(have, k)
		}
	}

	return have, nil
}
