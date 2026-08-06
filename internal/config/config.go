package config

import "os"

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func GithubAppID() string {
	return get("GITHUB_APP_ID", "invalid-dummy-id")
}

func GithubAppPrivateKey() string {
	return get("GITHUB_APP_PRIVATE_KEY", "invalid-dummy-secret")
}

func GithubAppInstallationID() string {
	return get("GITHUB_APP_INSTALLATION_ID", "22187127")
}

func PermissionsURL() string {
	return get("PERMISSIONS_URL", "https://ci.jenkins.io/job/Infra/job/repository-permissions-updater/job/master/lastSuccessfulBuild/artifact/json/github.index.json")
}

func JenkinsHost() string {
	return get("JENKINS_HOST", "https://ci.jenkins.io/")
}

func IncrementalURL() string {
	return get("INCREMENTAL_URL", "https://repo.jenkins-ci.org/incrementals/")
}

func ArtifactoryKey() string {
	return get("ARTIFACTORY_KEY", "invalid-key")
}

func JenkinsAuth() string {
	return get("JENKINS_AUTH", "")
}

func Port() string {
	return get("PORT", "3000")
}

func BuildMetadataURL() string {
	return get("BUILD_METADATA_URL", "")
}

func FolderMetadataURL() string {
	return get("FOLDER_METADATA_URL", "")
}

func ArchiveURL() string {
	return get("ARCHIVE_URL", "")
}

func PresharedKey() string {
	return get("PRESHARED_KEY", "")
}
