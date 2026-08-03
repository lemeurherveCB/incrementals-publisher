import {Octokit} from "@octokit/rest";
import {createAppAuth} from "@octokit/auth-app";

const APP_ID = process.env.GITHUB_APP_ID;
const PRIVATE_KEY = process.env.GITHUB_APP_PRIVATE_KEY;
const INSTALLATION_ID = process.env.GITHUB_APP_INSTALLATION_ID || 22187127;

export async function getRestClient() {
  return new Octokit({
    authStrategy: createAppAuth,
    auth: {
      appId: APP_ID,
      privateKey: PRIVATE_KEY,
      installationId: INSTALLATION_ID
    }
  });
}
