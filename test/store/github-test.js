import assert from "assert";
import {storeWithClient} from "../../lib/store/github.js";

const RAW = "name=foo-plugin:weekly;failCount=0;skipCount=0;passCount=8;totalCount=8;duration=15.298;elapsed=39.122;plugins=[foo];pluginCount=1;attempt=1;build_id=21;job_base_name=PR-1;short_commit_id=abc1234\n";

describe("GitHub store backend", function () {
  describe("storeWithClient", function () {
    it("writes both the build file and latest.txt", async function () {
      const err404 = Object.assign(new Error("Not Found"), {status: 404});
      const calls = [];
      const mockOctokit = {
        repos: {
          getContent: async () => { throw err404; },
          createOrUpdateFileContents: async (args) => { calls.push(args); return {}; }
        }
      };

      await storeWithClient(mockOctokit, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(calls.length, 2);
      assert.strictEqual(calls[0].path, "Plugins/bom/PR-1/21.txt");
      assert.strictEqual(calls[1].path, "Plugins/bom/PR-1/latest.txt");
    });

    it("stores identical content in both files", async function () {
      const err404 = Object.assign(new Error("Not Found"), {status: 404});
      const calls = [];
      const mockOctokit = {
        repos: {
          getContent: async () => { throw err404; },
          createOrUpdateFileContents: async (args) => { calls.push(args); return {}; }
        }
      };

      await storeWithClient(mockOctokit, "Plugins/bom/PR-1", "21", RAW);

      const decode = (c) => Buffer.from(c, "base64").toString("utf8");
      assert.strictEqual(decode(calls[0].content), RAW);
      assert.strictEqual(decode(calls[1].content), RAW);
    });

    it("creates a new build file when none exists (404)", async function () {
      const err404 = Object.assign(new Error("Not Found"), {status: 404});
      const calls = [];
      const mockOctokit = {
        repos: {
          getContent: async () => { throw err404; },
          createOrUpdateFileContents: async (args) => { calls.push(args); return {}; }
        }
      };

      await storeWithClient(mockOctokit, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(calls[0].sha, undefined);
    });

    it("overwrites an existing build file using its SHA", async function () {
      const calls = [];
      const mockOctokit = {
        repos: {
          getContent: async ({path}) => {
            if (path.endsWith("21.txt")) return {data: {sha: "buildsha", content: ""}};
            throw Object.assign(new Error("Not Found"), {status: 404});
          },
          createOrUpdateFileContents: async (args) => { calls.push(args); return {}; }
        }
      };

      await storeWithClient(mockOctokit, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(calls[0].sha, "buildsha");
      assert.strictEqual(calls[1].sha, undefined);
    });

    it("propagates non-404 errors from getContent", async function () {
      const err403 = Object.assign(new Error("Forbidden"), {status: 403});
      const mockOctokit = {
        repos: {
          getContent: async () => { throw err403; },
          createOrUpdateFileContents: async () => {}
        }
      };
      await assert.rejects(() => storeWithClient(mockOctokit, "Plugins/bom/PR-1", "21", RAW), {status: 403});
    });
  });
});
