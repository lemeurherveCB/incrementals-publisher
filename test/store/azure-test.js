import assert from "assert";
import {storeWithClient, getCredential, probe} from "../../lib/store/azure.js";
import {StorageSharedKeyCredential} from "@azure/storage-file-share";
import {DefaultAzureCredential} from "@azure/identity";

const RAW = "name=foo-plugin:weekly;failCount=0;passCount=8;totalCount=8;duration=15.298;elapsed=39.122\n";

function makeMockShareClient() {
  const writes = [];
  function makeDirClient(prefix) {
    return {
      createIfNotExists: async () => {},
      getDirectoryClient: (name) => makeDirClient(`${prefix}/${name}`),
      getFileClient: (name) => ({
        create: async () => {},
        uploadRange: async (content) => {
          writes.push({path: `${prefix}/${name}`, content: Buffer.from(content).toString("utf8")});
        }
      }),
    };
  }
  return {
    rootDirectoryClient: makeDirClient(""),
    writes,
  };
}

describe("Azure store backend", function () {
  describe("storeWithClient", function () {
    it("writes both the build file and latest.txt", async function () {
      const mockShare = makeMockShareClient();

      await storeWithClient(mockShare, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(mockShare.writes.length, 2);
      assert.ok(mockShare.writes.some(w => w.path.endsWith("21.txt")));
      assert.ok(mockShare.writes.some(w => w.path.endsWith("latest.txt")));
    });

    it("stores identical content in both files", async function () {
      const mockShare = makeMockShareClient();

      await storeWithClient(mockShare, "Plugins/bom/PR-1", "21", RAW);

      for (const w of mockShare.writes) {
        assert.strictEqual(w.content, RAW);
      }
    });

    it("calls create with the correct byte length", async function () {
      const lengths = [];
      function makeDirClient() {
        return {
          createIfNotExists: async () => {},
          getDirectoryClient: () => makeDirClient(),
          getFileClient: () => ({
            create: async (len) => { lengths.push(len); },
            uploadRange: async () => {}
          }),
        };
      }

      await storeWithClient({rootDirectoryClient: makeDirClient()}, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(lengths.length, 2);
      assert.ok(lengths.every(l => l === Buffer.from(RAW).length));
    });

    it("calls uploadRange with offset 0 and correct length", async function () {
      const uploadCalls = [];
      function makeDirClient() {
        return {
          createIfNotExists: async () => {},
          getDirectoryClient: () => makeDirClient(),
          getFileClient: () => ({
            create: async () => {},
            uploadRange: async (...args) => { uploadCalls.push(args); }
          }),
        };
      }

      await storeWithClient({rootDirectoryClient: makeDirClient()}, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(uploadCalls.length, 2);
      for (const [, offset, length] of uploadCalls) {
        assert.strictEqual(offset, 0);
        assert.strictEqual(length, Buffer.from(RAW).length);
      }
    });

    it("handles a flat job name (single directory, no nesting)", async function () {
      const mockShare = makeMockShareClient();

      await storeWithClient(mockShare, "PR-1", "21", RAW);

      assert.strictEqual(mockShare.writes.length, 2);
      assert.ok(mockShare.writes.some(w => w.path.endsWith("21.txt")));
      assert.ok(mockShare.writes.some(w => w.path.endsWith("latest.txt")));
    });
  });

  describe("probe", function () {
    it("is exported as a function", function () {
      assert.strictEqual(typeof probe, "function");
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
