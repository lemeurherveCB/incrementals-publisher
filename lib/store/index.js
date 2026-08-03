import config from "../config.js";

export async function createStore() {
  const backend = config.STORAGE_BACKEND;
  if (backend === "azure") {
    const {store} = await import("./azure.js");
    return {store};
  }
  if (backend === "github") {
    const {store} = await import("./github.js");
    return {store};
  }
  throw new Error(`Unknown STORAGE_BACKEND: "${backend}". Must be "github" or "azure".`);
}
