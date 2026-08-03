import assert from "assert";
import {storeWithClient} from "../../lib/store/azure.js";

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
});
