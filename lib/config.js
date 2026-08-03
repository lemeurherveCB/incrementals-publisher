const config = {};

Object.entries({
  STORAGE_BACKEND: "github",
  // GitHub backend
  GITHUB_APP_ID: "invalid-dummy-id",
  GITHUB_APP_PRIVATE_KEY: "invalid-dummy-secret",
  GITHUB_APP_INSTALLATION_ID: "",
  GITHUB_OWNER: "jenkins-infra",
  GITHUB_REPO: "results-publisher",
  // Azure backend
  AZURE_STORAGE_ACCOUNT: "",
  AZURE_STORAGE_SHARE: "",
  // Common
  PORT: "3000",
  PRESHARED_KEY: "",
}).forEach(([key, value]) => {
  Object.defineProperty(config, key, {
    get() {return process.env[key] || value},
    enumerable: true,
    configurable: false
  });
});

export default config;
