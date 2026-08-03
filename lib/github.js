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

export async function storeBuildResultsWithClient(octokit, jobName, buildId, rawText) {
  const owner = config.GITHUB_OWNER;
  const repo = config.GITHUB_REPO;
  const path = `${jobName}/${buildId}.txt`;

  let existingSha = undefined;
  try {
    const {data} = await octokit.repos.getContent({owner, repo, path});
    existingSha = data.sha;
  } catch (err) {
    if (err.status !== 404) throw err;
  }

  await octokit.repos.createOrUpdateFileContents({
    owner,
    repo,
    path,
    message: `chore: store BOM results for ${jobName} build ${buildId} [skip ci]`,
    content: Buffer.from(rawText).toString("base64"),
    sha: existingSha,
  });
}

export async function storeBuildResults(jobName, buildId, rawText) {
  const octokit = await getRestClient();
  return storeBuildResultsWithClient(octokit, jobName, buildId, rawText);
}
