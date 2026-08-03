import {S3Client, PutObjectCommand} from "@aws-sdk/client-s3";
import config from "../config.js";

export function getS3Client() {
  return new S3Client({region: config.AWS_REGION});
}

async function putFile(s3, key, rawText) {
  await s3.send(new PutObjectCommand({
    Bucket: config.AWS_S3_BUCKET,
    Key: key,
    Body: rawText,
    ContentType: "text/plain",
  }));
}

export async function storeWithClient(s3, jobName, buildId, rawText) {
  await putFile(s3, `${jobName}/${buildId}.txt`, rawText);
  await putFile(s3, `${jobName}/latest.txt`, rawText);
}

export async function store(jobName, buildId, rawText) {
  return storeWithClient(getS3Client(), jobName, buildId, rawText);
}
