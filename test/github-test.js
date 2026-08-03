import assert from "assert";
import {getRestClient, storeBuildResultsWithClient} from "../lib/github.js";

const RAW = "name=foo-plugin:weekly;failCount=0;skipCount=0;passCount=8;totalCount=8;duration=15.298;elapsed=39.122;plugins=[foo];pluginCount=1;attempt=1;build_id=21;job_base_name=PR-1;short_commit_id=abc1234\n";

describe("The GitHub helpers", function () {
  it("getRestClient is a function", function () {
    assert.strictEqual(typeof getRestClient, "function");
  });

  describe("storeBuildResultsWithClient", function () {
    it("creates a new file when none exists (404)", async function () {
      const err404 = Object.assign(new Error("Not Found"), {status: 404});
      let capturedArgs;
      const mockOctokit = {
        repos: {
          getContent: async () => { throw err404; },
          createOrUpdateFileContents: async (args) => { capturedArgs = args; return {}; }
        }
      };

      await storeBuildResultsWithClient(mockOctokit, "Plugins/bom/PR-1", "21", RAW);

      assert.ok(capturedArgs, "createOrUpdateFileContents should have been called");
      assert.strictEqual(capturedArgs.path, "Plugins/bom/PR-1/21.txt");
      assert.strictEqual(capturedArgs.sha, undefined);
      assert.strictEqual(Buffer.from(capturedArgs.content, "base64").toString("utf8"), RAW);
    });

    it("overwrites an existing file using its SHA", async function () {
      let capturedArgs;
      const mockOctokit = {
        repos: {
          getContent: async () => ({data: {sha: "existingsha123", content: ""}}),
          createOrUpdateFileContents: async (args) => { capturedArgs = args; return {}; }
        }
      };

      await storeBuildResultsWithClient(mockOctokit, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(capturedArgs.sha, "existingsha123");
      assert.strictEqual(capturedArgs.path, "Plugins/bom/PR-1/21.txt");
    });

    it("propagates non-404 errors from getContent", async function () {
      const err403 = Object.assign(new Error("Forbidden"), {status: 403});
      const mockOctokit = {
        repos: {
          getContent: async () => { throw err403; },
          createOrUpdateFileContents: async () => {}
        }
      };
      await assert.rejects(() => storeBuildResultsWithClient(mockOctokit, "Plugins/bom/PR-1", "21", RAW), {status: 403});
    });
  });
});
