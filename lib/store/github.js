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

async function putFile(octokit, path, content, message) {
  const owner = config.GITHUB_OWNER;
  const repo = config.GITHUB_REPO;

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
    message,
    content: Buffer.from(content).toString("base64"),
    sha: existingSha,
  });
}

export async function storeWithClient(octokit, jobName, buildId, rawText) {
  await putFile(
    octokit,
    `${jobName}/${buildId}.txt`,
    rawText,
    `chore: store BOM results for ${jobName} build ${buildId} [skip ci]`
  );
  await putFile(
    octokit,
    `${jobName}/latest.txt`,
    rawText,
    `chore: update latest BOM results for ${jobName} [skip ci]`
  );
}

export async function store(jobName, buildId, rawText) {
  const octokit = await getRestClient();
  return storeWithClient(octokit, jobName, buildId, rawText);
}
