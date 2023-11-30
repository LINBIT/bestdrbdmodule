package main

import (
	"bufio"
	"crypto/sha256"
	"flag"
	"fmt"
	"github.com/LINBIT/bestdrbdmodule/pkg/repos"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

var GitCommit string

const pkgUrl = "https://packages.linbit.com/yum/index.kmp.json"

var (
	flagAddr         = flag.String("addr", ":8080", "Server address")
	flagRepo         = flag.String("repo", "drbd-9", "Repository to use")
	flagDists        = flag.String("dist", "rhel7,rhel8,rhel9", "Distributions")
	flagFetch        = flag.Duration("fetch", 20*time.Minute, "Package DB fetch interval")
	flagMaxBytesBody = flag.Int("maxbytesbody", 250*1024, "Maximum nunber of bytes in the body")
	flagCertFile     = flag.String("certfile", "", "Path to a TLS cert file")
	flagKeyFile      = flag.String("keyfile", "", "Path to a TLS key file")
	flagDebug        = flag.Bool("debug", false, "Enable debug logging (otherwise production level log)")
	flagVersion      = flag.Bool("version", false, "Print version and exit")
)

type server struct {
	router       *mux.Router
	versions     map[string]versionMap
	kmodCache    map[string]moduleCache
	maxBytesBody int64
	logger       *zap.Logger

	m sync.Mutex
}

type moduleCache map[[32]byte]string

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Printf("Git-commit: '%s'\n", GitCommit)
		os.Exit(0)
	}

	if *flagMaxBytesBody < 0 {
		log.Fatal("maxbytesbody has to be a positive value")
	}

	s := &server{
		router:       mux.NewRouter(),
		kmodCache:    make(map[string]moduleCache),
		maxBytesBody: int64(*flagMaxBytesBody),
		versions:     make(map[string]versionMap),
	}

	// additional setup
	var err error
	if *flagDebug {
		s.logger, err = zap.NewDevelopment()
	} else {
		s.logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal(err)
	}

	s.routes()
	// try to initially fill kmods before we serve the first request
	s.updateKmps(*flagRepo, *flagDists)
	go s.repoWatcher(*flagRepo, *flagDists, *flagFetch)

	server := http.Server{
		Addr:           *flagAddr,
		Handler:        s,
		MaxHeaderBytes: 4 * 1024,
	}

	if *flagCertFile != "" && *flagKeyFile != "" {
		log.Fatal(server.ListenAndServeTLS(*flagCertFile, *flagKeyFile))
	} else {
		log.Fatal(server.ListenAndServe())
	}
}

// handler interface, wrapped for MaxBytesReader
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBytesBody)
	s.router.ServeHTTP(w, r)
}

func (s *server) hello() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/text")

		if _, err := fmt.Fprintf(w, "Successfully connected to bestkernelmatch ('%s')\n", GitCommit); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func (s *server) bestKmod() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/text")

		kernelRelease, ok := mux.Vars(r)["kernelrelease"]
		if !ok || kernelRelease == "" || len(kernelRelease) > 42 {
			s.errorf(http.StatusBadRequest, r.RemoteAddr, w, "Could not get valid kernelrelease parameter")
			return
		}

		// this piece of fine work called centos does not properly set version information in os-release
		// and they don't give a damn, so we can not use lbdisttool without /etc/centos_release
		// BUT for kmod in this context we don't care about the centos dot release, so we actually can use
		// their broken os-release and then use disttool with a forced version
		h := sha256.New()
		if _, err := h.Write([]byte(kernelRelease)); err != nil {
			s.errorf(http.StatusInternalServerError, r.RemoteAddr, w, "Could not hash input line")
			return
		}
		dist, major := "", ""
		scanner := bufio.NewScanner(r.Body)
		for scanner.Scan() {
			text := scanner.Text()

			// hash early/everything
			// do not break out of this loop
			if _, err := h.Write([]byte(text)); err != nil {
				s.errorf(http.StatusInternalServerError, r.RemoteAddr, w, "Could not hash input line")
				return
			}

			text = strings.TrimSpace(text)
			if strings.HasPrefix(text, "#") {
				continue
			}
			kv := strings.SplitN(text, "=", 2)
			if len(kv) < 2 {
				continue
			}
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			if k == "ID" {
				dist = osreleaseValue(v)
			} else if k == "VERSION_ID" {
				v = osreleaseValue(v)
				majorminor := strings.SplitN(v, ".", 2)
				if len(majorminor) == 0 {
					continue
				}
				major = majorminor[0]
			}
		}
		if err := scanner.Err(); err != nil {
			s.errorf(http.StatusBadRequest, r.RemoteAddr, w, "Scanner error while processing your os-release: %v", err)
			return
		}
		hSum := sha256.Sum256(h.Sum(nil))

		// mappings
		if dist == "centos" || dist == "almalinux" || dist == "rocky" {
			dist = "rhel"
		}

		if dist == "" {
			s.errorf(http.StatusBadRequest, r.RemoteAddr, w, "Could not determine distribution from os-release")
			return
		}
		if major == "" {
			s.errorf(http.StatusBadRequest, r.RemoteAddr, w, "Could not determine major release from os-release")
			return
		}

		dist = dist + major
		if hit, ok := s.getKmodCache(dist, hSum); ok {
			s.logger.Info("Cache hit for: " + hit)
			if _, err := fmt.Fprint(w, hit); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			return
		}

		s.m.Lock()
		defer s.m.Unlock()
		versions, ok := s.versions[dist]
		if !ok {
			s.errorf(http.StatusBadRequest, r.RemoteAddr, w, "Distribution '%v' not supported", dist)
			return
		} else if len(versions) == 0 {
			s.errorf(http.StatusInternalServerError, r.RemoteAddr, w, "No versions for distribution '%v'", dist)
			return
		}

		lbdisttool, err := exec.LookPath("lbdisttool.py")
		if err != nil {
			s.errorf(http.StatusInternalServerError, r.RemoteAddr, w, "lbdistool: %v", err)
			return
		}

		args := []string{
			lbdisttool,
			"--force-name", dist,
			"--force-kernel-release", kernelRelease,
			"-k",
		}
		for _, v := range versions {
			args = append(args, v.pkg)
		}
		s.logger.Info("python3 " + strings.Join(args, " "))
		out, err := exec.Command("python3", args...).Output()
		if err != nil {
			s.errorf(http.StatusBadRequest, r.RemoteAddr, w, "lbdistool: %v", err)
			return
		}

		// if there is no hit, lbdisttool errors out.
		hit := strings.TrimSpace(string(out))
		s._setKmodCache(dist, hSum, hit)
		if _, err := fmt.Fprint(w, hit); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func (s *server) updateKmps(repo, dists string) {
	repoInfo, err := repos.Get(pkgUrl)
	if err != nil { // not fatal, we just can not offer anything
		s.logger.Info("Could not fetch repo info: " + err.Error())
	}

	for _, d := range strings.Split(dists, ",") {
		kmps := repoInfo.GetKmps(repo, d, "amd64")
		if len(kmps) == 0 {
			continue
		}
		vmap, err := filterKmps(kmps)
		if err != nil {
			s.logger.Info("filterKmps: " + err.Error())
			continue
		}
		s.m.Lock()
		s.versions[d] = vmap
		// invalidate cache
		s.kmodCache[d] = make(moduleCache)
		s.m.Unlock()
	}
}

func (s *server) repoWatcher(repo, dists string, interval time.Duration) {
	for {
		s.updateKmps(repo, dists)
		time.Sleep(interval)
	}
}

func (s *server) getKmodCache(dist string, hSum [32]byte) (string, bool) {
	s.m.Lock()
	defer s.m.Unlock()

	c, ok := s.kmodCache[dist]
	if !ok {
		return "", false
	}
	v, ok := c[hSum]
	return v, ok
}

// needs to hold the lock
func (s *server) _setKmodCache(dist string, hSum [32]byte, value string) {
	_, ok := s.kmodCache[dist]
	if !ok {
		s.kmodCache[dist] = make(moduleCache)
	}

	s.kmodCache[dist][hSum] = value
}

// func (s *server) setKmodCache(dist string, hSum [32]byte, value string) {
// s.m.Lock()
// s._setKmodCache(dist, hSum, value)
// s.m.Unlock()
// }

func (s *server) errorf(code int, remoteAddr string, w http.ResponseWriter, format string, a ...interface{}) {
	w.WriteHeader(code)
	_, _ = fmt.Fprintf(w, format, a...)
	s.logger.Error(fmt.Sprintf(format, a...),
		zap.String("type", "error"),
		zap.String("remoteAddr", remoteAddr),
		zap.Int("code", code))
}

// does not handle things like asym quotes but good enough...
func osreleaseValue(s string) string {
	if len(s) > 0 && (s[0] == '"' || s[0] == '\'') {
		s = s[1:]
	}
	if len(s) > 0 && (s[len(s)-1] == '"' || s[len(s)-1] == '\'') {
		s = s[:len(s)-1]
	}

	return s
}
