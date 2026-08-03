import {Octokit} from "@octokit/rest";
import {createAppAuth} from "@octokit/auth-app";
import config from "./config.js";

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

export async function storeResultsWithClient(octokit, entries) {
  const owner = config.GITHUB_OWNER;
  const repo = config.GITHUB_REPO;
  const path = config.RESULTS_FILE_PATH;

  let existingSha = undefined;
  let existingContent = "";

  try {
    const {data} = await octokit.repos.getContent({owner, repo, path});
    existingSha = data.sha;
    existingContent = Buffer.from(data.content, "base64").toString("utf8");
  } catch (err) {
    if (err.status !== 404) throw err;
  }

  const recorded_at = new Date().toISOString();
  const newLines = entries.map(entry => JSON.stringify({...entry, recorded_at})).join("\n") + "\n";
  const updatedContent = existingContent + newLines;

  await octokit.repos.createOrUpdateFileContents({
    owner,
    repo,
    path,
    message: `chore: add ${entries.length} BOM test result(s) [skip ci]`,
    content: Buffer.from(updatedContent).toString("base64"),
    sha: existingSha,
  });
}

export async function storeResults(entries) {
  const octokit = await getRestClient();
  return storeResultsWithClient(octokit, entries);
}
