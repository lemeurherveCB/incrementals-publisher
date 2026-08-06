package main

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	gogithub "github.com/jenkins-infra/incrementals-publisher/internal/github"
	"github.com/jenkins-infra/incrementals-publisher/internal/config"
	"github.com/jenkins-infra/incrementals-publisher/internal/permissions"
	"github.com/jenkins-infra/incrementals-publisher/internal/pipeline"
)

const version = "1.4.2"

// buildURLPathRe validates that the path follows /job/<name>/.../N/ structure.
// Each job segment: alphanumeric, dots, hyphens, underscores only.
// Build number: digits only. Must end with exactly one slash.
var buildURLPathRe = regexp.MustCompile(`^(/job/[a-zA-Z0-9._-]+)+/[0-9]+/$`)

// httpClient is the shared client used for Jenkins and Artifactory calls.
// The JS version used node-fetch without explicit timeouts in the main path;
// we set one here to avoid hanging handler goroutines on slow upstreams.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// readinessHTTPClient is the dedicated client for the /readiness Jenkins probe.
var readinessHTTPClient = &http.Client{Timeout: 5 * time.Second}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Fail fast if PRESHARED_KEY is empty — would otherwise admit any caller.
	if config.PresharedKey() == "" {
		log.Error("PRESHARED_KEY is not set; refusing to start")
		os.Exit(1)
	}

	ghClient, err := gogithub.NewClient(config.GithubAppID(), config.GithubAppPrivateKey(), config.GithubAppInstallationID())
	if err != nil {
		log.Error("failed to create GitHub client", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /liveness", livenessHandler(log))
	mux.HandleFunc("GET /readiness", readinessHandler(log))
	mux.HandleFunc("POST /", publishHandler(log, ghClient))

	addr := ":" + config.Port()
	srv := &http.Server{
		Addr:         addr,
		Handler:      requestLogger(log, mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("incrementals-publisher listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func livenessHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(log, w, http.StatusOK, map[string]string{"status": "OK", "version": version})
	}
}

func readinessHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type response struct {
			Errors  []string `json:"errors"`
			Jenkins string   `json:"jenkins,omitempty"`
		}
		resp := response{}
		status := http.StatusOK

		jenkinsAuth := config.JenkinsAuth()
		if jenkinsAuth == "" {
			resp.Jenkins = "no_auth"
		} else {
			req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(config.JenkinsHost(), "/")+"/whoAmI/api/json", nil)
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(jenkinsAuth)))
			res, err := readinessHTTPClient.Do(req)
			if err != nil || res.StatusCode != http.StatusOK {
				log.Error("Jenkins healthcheck failed", "error", err)
				resp.Errors = append(resp.Errors, "jenkins")
				status = http.StatusInternalServerError
			} else {
				resp.Jenkins = "ok"
				res.Body.Close()
			}
		}

		writeJSON(log, w, status, resp)
	}
}

type publishRequest struct {
	BuildURL string `json:"build_url"`
}

// githubClient is the subset of gogithub.Client used by publishHandler,
// defined as an interface so tests can inject a stub without a real GitHub App.
type githubClient interface {
	CommitExists(ctx context.Context, owner, repo, ref string) (bool, error)
	CreateCheckRun(ctx context.Context, owner, repo, headSHA string, entries []gogithub.ArtifactEntry) error
}

func publishHandler(log *slog.Logger, ghClient githubClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// --- Authentication ---
		// Use constant-time comparison to prevent timing attacks (JS used bcrypt for the same reason).
		authHeader := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(authHeader), []byte(config.PresharedKey())) != 1 {
			http.Error(w, "Not authorized", http.StatusForbidden)
			return
		}

		var body publishRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		if body.BuildURL == "" {
			http.Error(w, "The incrementals-publisher invocation was missing the build_url attribute", http.StatusBadRequest)
			return
		}

		if err := validateBuildURL(body.BuildURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Fetch permissions early (can happen concurrently with Jenkins calls).
		permsCh := make(chan struct {
			perms map[string][]string
			err   error
		}, 1)
		go func() {
			p, err := permissions.FetchPermissions(config.PermissionsURL())
			permsCh <- struct {
				perms map[string][]string
				err   error
			}{p, err}
		}()

		// --- Jenkins metadata ---
		jenkinsHeaders := jenkinsAuthHeader()

		buildMetaURL := config.BuildMetadataURL()
		if buildMetaURL == "" {
			buildMetaURL = pipeline.GetBuildAPIURL(body.BuildURL)
		}
		buildData, err := fetchJSON(ctx, buildMetaURL, jenkinsHeaders)
		if err != nil {
			log.Error("failed to fetch build metadata", "error", err)
			http.Error(w, "failed to fetch build metadata", http.StatusBadGateway)
			return
		}

		buildMeta := pipeline.ProcessBuildMetadata(buildData)
		if buildMeta.Hash == "" {
			msg := fmt.Sprintf("Did not find a Git commit hash associated with this build. Some plugins on %s may not yet have been updated with JENKINS-50777 REST API enhancements. Skipping deployment.\n", config.JenkinsHost())
			log.Warn(msg)
			fmt.Fprint(w, msg)
			return
		}

		folderMetaURL := config.FolderMetadataURL()
		if folderMetaURL == "" {
			folderMetaURL = pipeline.GetFolderAPIURL(body.BuildURL)
		}
		folderData, err := fetchJSON(ctx, folderMetaURL, jenkinsHeaders)
		if err != nil {
			log.Error("failed to fetch folder metadata", "error", err)
			http.Error(w, "failed to fetch folder metadata", http.StatusBadGateway)
			return
		}

		folderMeta := pipeline.ProcessFolderMetadata(folderData)
		if folderMeta.Owner == "" || folderMeta.Repo == "" {
			http.Error(w, "Unable to retrieve an owner or repo", http.StatusBadRequest)
			return
		}

		// --- GitHub commit verification ---
		exists, err := ghClient.CommitExists(ctx, folderMeta.Owner, folderMeta.Repo, buildMeta.Hash)
		if err != nil || !exists {
			log.Error("commit does not exist or could not be verified", "hash", buildMeta.Hash, "error", err)
			http.Error(w, "Could not find commit (non-existent or ambiguous)", http.StatusBadRequest)
			return
		}
		log.Info("metadata loaded", "repo", folderMeta.Owner+"/"+folderMeta.Repo, "hash", buildMeta.Hash)

		// --- Download archive ---
		archiveURL := config.ArchiveURL()
		if archiveURL == "" {
			archiveURL = pipeline.GetArchiveURL(body.BuildURL, buildMeta.Hash)
		}
		archivePath, cleanup, err := downloadToTemp(ctx, archiveURL, jenkinsHeaders)
		if err != nil {
			log.Error("failed to download archive", "url", archiveURL, "error", err)
			http.Error(w, "failed to download archive", http.StatusBadGateway)
			return
		}
		defer cleanup()
		log.Info("downloaded archive", "url", archiveURL, "path", archivePath)

		// --- Wait for permissions ---
		permsResult := <-permsCh
		if permsResult.err != nil {
			log.Error("failed to fetch permissions", "error", permsResult.err)
			http.Error(w, "Failed to retrieve permissions", http.StatusBadRequest)
			return
		}

		var entries []permissions.Entry
		repoPath := folderMeta.Owner + "/" + folderMeta.Repo
		if err := permissions.Verify(log, repoPath, archivePath, &entries, permsResult.perms, buildMeta.Hash); err != nil {
			log.Error("invalid archive", "error", err)
			http.Error(w, fmt.Sprintf("Invalid archive retrieved from Jenkins, perhaps the plugin is not properly incrementalized?\n%v from %s", err, archiveURL), http.StatusBadRequest)
			return
		}

		if len(entries) == 0 {
			msg := fmt.Sprintf("Skipping deployment as no artifacts were found with the expected path, typically due to a PR merge build not up to date with its base branch: %s\n", archiveURL)
			log.Warn(msg)
			fmt.Fprint(w, msg)
			return
		}
		log.Info("archive entries validated", "count", len(entries))

		// --- Idempotency check ---
		pom := entries[0].Path
		pomURL := config.IncrementalURL() + pom
		checkResp, err := http.Get(pomURL) //nolint:gosec // URL is constructed from trusted config + validated archive path
		if err == nil {
			checkResp.Body.Close()
			if checkResp.StatusCode == http.StatusOK {
				msg := fmt.Sprintf("Already deployed, not attempting to redeploy: %s\n", pomURL)
				log.Info(msg)
				fmt.Fprint(w, msg)
				return
			}
		}

		// --- Upload to Artifactory ---
		uploadStatus, uploadText, err := uploadToArtifactory(ctx, archivePath)
		if err != nil {
			log.Error("upload failed", "error", err)
			http.Error(w, "upload to Artifactory failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		log.Info("upload result", "pomURL", pomURL, "status", uploadStatus)

		// --- GitHub Check Run ---
		var displayEntries []gogithub.ArtifactEntry
		for _, e := range entries {
			dir := e.Path[:strings.LastIndex(e.Path, "/")+1]
			displayEntries = append(displayEntries, gogithub.ArtifactEntry{
				ArtifactID: e.ArtifactID,
				GroupID:    e.GroupID,
				Version:    e.Version,
				Packaging:  e.Packaging,
				URL:        config.IncrementalURL() + dir,
			})
		}
		if err := ghClient.CreateCheckRun(ctx, folderMeta.Owner, folderMeta.Repo, buildMeta.Hash, displayEntries); err != nil {
			log.Error("failed to create GitHub check run", "error", err)
			// Non-fatal — log and continue.
		} else {
			log.Info("created GitHub check run", "pom", pom)
		}

		if uploadStatus >= 300 {
			http.Error(w, fmt.Sprintf("Artifactory returned %d: %s", uploadStatus, uploadText), uploadStatus)
			return
		}
		fmt.Fprintf(w, "Response from Artifactory: %s\n", uploadText)
	}
}

// validateBuildURL checks that buildURL belongs to JENKINS_HOST and has a valid path.
// Error message text is preserved from the JS version (IncrementalsPlugin.isValidUrl)
// because it is part of the HTTP response body and callers may depend on it.
func validateBuildURL(buildURL string) error {
	parsed, err := url.Parse(buildURL)
	if err != nil {
		return fmt.Errorf("This build_url is malformed") //nolint:stylecheck
	}

	jenkinsHost, _ := url.Parse(config.JenkinsHost())
	if parsed.Scheme+"://"+parsed.Host != jenkinsHost.Scheme+"://"+jenkinsHost.Host {
		return fmt.Errorf("This build_url is not supported") //nolint:stylecheck
	}

	// Reject raw path traversal sequences before regex validation.
	rawPath := parsed.RawPath
	if rawPath == "" {
		rawPath = parsed.Path
	}
	if strings.Contains(rawPath, "/../") || strings.Contains(rawPath, "/./") ||
		strings.Contains(rawPath, "%") || // percent-encoded chars disallowed
		strings.Contains(rawPath, "?") || strings.Contains(rawPath, "#") ||
		strings.Contains(rawPath, "//") {
		return fmt.Errorf("This build_url is malformed") //nolint:stylecheck
	}

	if !buildURLPathRe.MatchString(parsed.Path) {
		return fmt.Errorf("This build_url is malformed") //nolint:stylecheck
	}
	return nil
}

func jenkinsAuthHeader() map[string]string {
	auth := config.JenkinsAuth()
	if auth == "" {
		return nil
	}
	return map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(auth)),
	}
}

func fetchJSON(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(resp.Body)
}

func downloadToTemp(ctx context.Context, rawURL string, headers map[string]string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "incrementals-*.zip")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(f.Name())
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("HTTP %d downloading archive", resp.StatusCode)
	}

	if _, err = io.Copy(f, resp.Body); err != nil {
		return "", nil, fmt.Errorf("writing archive: %w", err)
	}
	if err = f.Close(); err != nil {
		return "", nil, fmt.Errorf("closing temp file: %w", err)
	}

	name := f.Name()
	return name, func() { os.Remove(name) }, nil
}

// uploadToArtifactory always uploads to config.INCREMENTAL_URL/archive.zip
// using Artifactory's X-Explode-Archive header — same as the JS version which
// also ignored pomURL and used config.INCREMENTAL_URL + "archive.zip" directly.
func uploadToArtifactory(ctx context.Context, archivePath string) (int, string, error) {
	uploadURL := config.IncrementalURL() + "archive.zip"
	f, err := os.Open(archivePath)
	if err != nil {
		return 0, "", fmt.Errorf("opening archive: %w", err)
	}
	defer f.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, f)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("X-Explode-Archive", "true")
	req.Header.Set("X-Explode-Archive-Atomic", "true")
	req.Header.Set("X-JFrog-Art-Api", config.ArtifactoryKey())

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("uploading to Artifactory: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// Log body but don't surface as error — caller checks status code.
	return resp.StatusCode, resp.Status + "\n" + string(body), nil
}

func writeJSON(log *slog.Logger, w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error("failed to write JSON response", "error", err)
	}
}

func requestLogger(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
