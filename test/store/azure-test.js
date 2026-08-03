import assert from "assert";
import {storeWithClient, getCredential} from "../../lib/store/azure.js";
import {StorageSharedKeyCredential} from "@azure/storage-file-share";
import {DefaultAzureCredential} from "@azure/identity";

const RAW = "name=foo-plugin:weekly;failCount=0;passCount=8;totalCount=8;duration=15.298;elapsed=39.122\n";

function makeMockShareClient(overrides = {}) {
  const fileClient = {
    create: async () => {},
    uploadRange: async (...args) => { fileClient._lastUpload = args; },
    ...overrides.fileClient
  };
  function makeDirClient() {
    return {
      createIfNotExists: async () => {},
      getDirectoryClient: () => makeDirClient(),
      getFileClient: () => fileClient,
    };
  }
  return {
    rootDirectoryClient: makeDirClient(),
    _fileClient: fileClient,
  };
}

describe("Azure store backend", function () {
  describe("storeWithClient", function () {
    it("creates parent directories and uploads file content", async function () {
      const mockShare = makeMockShareClient();

      await storeWithClient(mockShare, "Plugins/bom/PR-1", "21", RAW);

      const uploaded = Buffer.from(mockShare._fileClient._lastUpload[0]).toString("utf8");
      assert.strictEqual(uploaded, RAW);
    });

    it("derives the correct file path from jobName and buildId", async function () {
      let capturedFileName;
      const fileClient = {
        create: async () => {},
        uploadRange: async () => {}
      };
      function makeDirClient() {
        return {
          createIfNotExists: async () => {},
          getDirectoryClient: () => makeDirClient(),
          getFileClient: (name) => { capturedFileName = name; return fileClient; },
        };
      }
      const mockShare = {rootDirectoryClient: makeDirClient()};

      await storeWithClient(mockShare, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(capturedFileName, "21.txt");
    });
  });

  describe("getCredential", function () {
    const original = process.env.AZURE_STORAGE_KEY;

    afterEach(function () {
      if (original === undefined) {
        delete process.env.AZURE_STORAGE_KEY;
      } else {
        process.env.AZURE_STORAGE_KEY = original;
      }
    });

    it("returns DefaultAzureCredential when AZURE_STORAGE_KEY is unset", function () {
      delete process.env.AZURE_STORAGE_KEY;
      assert.ok(getCredential() instanceof DefaultAzureCredential);
    });

    it("returns StorageSharedKeyCredential when AZURE_STORAGE_KEY is set", function () {
      process.env.AZURE_STORAGE_ACCOUNT = "myaccount";
      process.env.AZURE_STORAGE_KEY = "mykey";
      assert.ok(getCredential() instanceof StorageSharedKeyCredential);
    });
  });
});
