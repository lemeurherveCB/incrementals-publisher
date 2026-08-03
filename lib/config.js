const config = {};

Object.entries({
  AZURE_STORAGE_ACCOUNT: "",
  AZURE_STORAGE_KEY: "",
  AZURE_STORAGE_SHARE: "",
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
