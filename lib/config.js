const config = {};

Object.entries({
  GITHUB_APP_ID: "invalid-dummy-id",
  GITHUB_APP_PRIVATE_KEY: "invalid-dummy-secret",
  GITHUB_APP_INSTALLATION_ID: "",
  GITHUB_OWNER: "jenkins-infra",
  GITHUB_REPO: "results-publisher",
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
