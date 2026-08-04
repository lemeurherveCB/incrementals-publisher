package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/fileerror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/share"
)

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				return s.Value[:7]
			}
		}
	}
	return "dev"
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	presharedKey := os.Getenv("PRESHARED_KEY")
	if presharedKey == "" {
		log.Fatal("PRESHARED_KEY is required")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /liveness", handleLiveness)
	mux.HandleFunc("GET /readiness", handleReadiness)
	mux.HandleFunc("POST /bom-results", handleBomResults([]byte(presharedKey)))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      secureHeaders(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("BOM results publisher listening at http://localhost:%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// --- handlers ---

func handleLiveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "OK", "version": buildVersion()})
}

func handleReadiness(w http.ResponseWriter, r *http.Request) {
	if err := probe(r.Context()); err != nil {
		log.Printf("readiness probe failed: %v", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "OK"})
}

func handleBomResults(presharedKey []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r, presharedKey) {
			http.Error(w, "Not authorized", http.StatusForbidden)
			return
		}

		var body struct {
			BuildURL string `json:"build_url"`
			Results  string `json:"results"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
		if body.BuildURL == "" || body.Results == "" {
			http.Error(w, "Missing required fields: build_url, results", http.StatusBadRequest)
			return
		}
		controller, jobName, buildID, err := parseBuildURL(body.BuildURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateController(controller); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateJobName(jobName); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateBuildID(buildID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("Storing results for %s/%s build %s", controller, jobName, buildID)
		if err := storeResults(r.Context(), controller, jobName, buildID, body.Results); err != nil {
			log.Printf("store error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "OK")
	}
}

// --- Azure storage ---

func getShareClient() (*share.Client, error) {
	account := os.Getenv("AZURE_STORAGE_ACCOUNT")
	shareName := os.Getenv("AZURE_STORAGE_SHARE")
	shareURL := fmt.Sprintf("https://%s.file.core.windows.net/%s", account, shareName)

	if key := os.Getenv("AZURE_STORAGE_KEY"); key != "" {
		cred, err := share.NewSharedKeyCredential(account, key)
		if err != nil {
			return nil, err
		}
		return share.NewClientWithSharedKeyCredential(shareURL, cred, nil)
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return share.NewClient(shareURL, cred, nil)
}

func storeResults(ctx context.Context, controller, jobName, buildID, rawText string) error {
	shareClient, err := getShareClient()
	if err != nil {
		return fmt.Errorf("get share client: %w", err)
	}
	content := []byte(rawText)
	base := controller + "/" + jobName
	if err := putFile(ctx, shareClient, fmt.Sprintf("%s/%s.txt", base, buildID), content); err != nil {
		return err
	}
	return putFile(ctx, shareClient, fmt.Sprintf("%s/latest.txt", base), content)
}

func putFile(ctx context.Context, shareClient *share.Client, filePath string, content []byte) error {
	dirClient := shareClient.NewRootDirectoryClient()
	for _, part := range strings.Split(path.Dir(filePath), "/") {
		dirClient = dirClient.NewSubdirectoryClient(part)
		if _, err := dirClient.Create(ctx, nil); err != nil && !fileerror.HasCode(err, fileerror.ResourceAlreadyExists) {
			return fmt.Errorf("create directory %q: %w", part, err)
		}
	}

	if err := dirClient.NewFileClient(path.Base(filePath)).UploadBuffer(ctx, content, nil); err != nil {
		return fmt.Errorf("upload file %q: %w", path.Base(filePath), err)
	}
	return nil
}

func probe(ctx context.Context) error {
	shareClient, err := getShareClient()
	if err != nil {
		return err
	}
	_, err = shareClient.GetProperties(ctx, nil)
	return err
}

// --- validation ---

var (
	reJobName  = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)
	reBuildID  = regexp.MustCompile(`^[0-9]+$`)
	reHostname = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
)

// parseBuildURL extracts (controller, jobName, buildID) from a Jenkins BUILD_URL.
// Expected format: http://host[:port][/prefix]/job/NAME[/job/SEGMENT]*/NUMBER/
func parseBuildURL(rawURL string) (controller, jobName, buildID string, err error) {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("build_url is not a valid URL")
	}
	controller = u.Hostname()

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	var jobParts []string
	i := 0
	for i < len(segments) {
		if segments[i] == "job" && i+1 < len(segments) {
			jobParts = append(jobParts, segments[i+1])
			i += 2
		} else {
			i++
		}
	}
	if len(jobParts) == 0 {
		return "", "", "", fmt.Errorf("build_url: no /job/ segment found")
	}
	// Last element may be the build number; check if last job part is numeric.
	// Jenkins paths end with …/job/NAME/NUMBER/ so the build number is the segment
	// immediately after the final job/NAME pair.
	last := segments[len(segments)-1]
	if last == "" && len(segments) > 1 {
		last = segments[len(segments)-2]
	}
	if reBuildID.MatchString(last) {
		buildID = last
	} else {
		return "", "", "", fmt.Errorf("build_url: no build number found")
	}
	jobName = strings.Join(jobParts, "/")
	return controller, jobName, buildID, nil
}

func validateJobName(s string) error {
	if !reJobName.MatchString(s) {
		return fmt.Errorf("job_name contains invalid characters")
	}
	if strings.Contains(s, "..") {
		return fmt.Errorf("job_name must not contain '..'")
	}
	return nil
}

func validateBuildID(s string) error {
	if !reBuildID.MatchString(s) {
		return fmt.Errorf("build_id must be numeric")
	}
	return nil
}

func validateController(s string) error {
	if !reHostname.MatchString(s) {
		return fmt.Errorf("build_url: controller hostname contains invalid characters")
	}
	return nil
}

// --- helpers ---

func checkAuth(r *http.Request, presharedKey []byte) bool {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), presharedKey) == 1
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
}
