import {Octokit} from "@octokit/rest";
import {createAppAuth} from "@octokit/auth-app";
import config from "../config.js";

async function getRestClient() {
  return new Octokit({
    authStrategy: createAppAuth,
    auth: {
      appId: config.GITHUB_APP_ID,
      privateKey: config.GITHUB_APP_PRIVATE_KEY,
      installationId: config.GITHUB_APP_INSTALLATION_ID || 22187127
    }
  });
}

export async function storeWithClient(octokit, jobName, buildId, rawText) {
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

export async function store(jobName, buildId, rawText) {
  const octokit = await getRestClient();
  return storeWithClient(octokit, jobName, buildId, rawText);
}
