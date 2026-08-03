import {ShareServiceClient, StorageSharedKeyCredential} from "@azure/storage-file-share";
import {DefaultAzureCredential} from "@azure/identity";
import config from "../config.js";

function getShareClient() {
  const credential = config.AZURE_STORAGE_KEY
    ? new StorageSharedKeyCredential(config.AZURE_STORAGE_ACCOUNT, config.AZURE_STORAGE_KEY)
    : new DefaultAzureCredential();
  const serviceClient = new ShareServiceClient(
    `https://${config.AZURE_STORAGE_ACCOUNT}.file.core.windows.net`,
    credential
  );
  return serviceClient.getShareClient(config.AZURE_STORAGE_SHARE);
}

export async function storeWithClient(shareClient, jobName, buildId, rawText) {
  const path = `${jobName}/${buildId}.txt`;
  const parts = path.split("/");
  const fileName = parts.pop();

  // ensure all parent directories exist
  let dirClient = shareClient.rootDirectoryClient;
  for (const part of parts) {
    dirClient = dirClient.getDirectoryClient(part);
    await dirClient.createIfNotExists();
  }

  const fileClient = dirClient.getFileClient(fileName);
  const content = Buffer.from(rawText);
  await fileClient.create(content.length);
  await fileClient.uploadRange(content, 0, content.length);
}

export async function store(jobName, buildId, rawText) {
  return storeWithClient(getShareClient(), jobName, buildId, rawText);
}
