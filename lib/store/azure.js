import {ShareServiceClient, StorageSharedKeyCredential} from "@azure/storage-file-share";
import {DefaultAzureCredential} from "@azure/identity";
import config from "../config.js";

export function getCredential() {
  return config.AZURE_STORAGE_KEY
    ? new StorageSharedKeyCredential(config.AZURE_STORAGE_ACCOUNT, config.AZURE_STORAGE_KEY)
    : new DefaultAzureCredential();
}

function getShareClient() {
  const credential = getCredential();
  const serviceClient = new ShareServiceClient(
    `https://${config.AZURE_STORAGE_ACCOUNT}.file.core.windows.net`,
    credential
  );
  return serviceClient.getShareClient(config.AZURE_STORAGE_SHARE);
}

async function putFile(shareClient, filePath, rawText) {
  const parts = filePath.split("/");
  const fileName = parts.pop();

  let dirClient = shareClient.rootDirectoryClient;
  for (const part of parts) {
    dirClient = dirClient.getDirectoryClient(part);
    await dirClient.createIfNotExists();
  }

  const content = Buffer.from(rawText);
  const fileClient = dirClient.getFileClient(fileName);
  await fileClient.create(content.length);
  await fileClient.uploadRange(content, 0, content.length);
}

export async function storeWithClient(shareClient, jobName, buildId, rawText) {
  await putFile(shareClient, `${jobName}/${buildId}.txt`, rawText);
  await putFile(shareClient, `${jobName}/latest.txt`, rawText);
}

export async function store(jobName, buildId, rawText) {
  return storeWithClient(getShareClient(), jobName, buildId, rawText);
}

export async function probe() {
  await getShareClient().getProperties();
}
