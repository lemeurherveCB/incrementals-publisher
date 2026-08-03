import assert from "assert";
import {getRestClient} from "../lib/github.js";
import {storeResultsWithClient} from "../lib/github.js";

const SAMPLE_ENTRIES = [
  {
    name: "foo-plugin:weekly",
    failCount: 0,
    skipCount: 0,
    passCount: 8,
    totalCount: 8,
    duration: 15.298,
    elapsed: 39.122,
    plugins: ["foo"],
    pluginCount: 1,
    attempt: 1,
    build_id: 21,
    job_base_name: "PR-1",
    short_commit_id: "abc1234"
  }
];

function makeMockOctokit({getContentResult, getContentError} = {}) {
  return {
    repos: {
      getContent: async () => {
        if (getContentError) throw getContentError;
        return getContentResult;
      },
      createOrUpdateFileContents: async (args) => {
        makeMockOctokit._lastCreateArgs = args;
        return {};
      }
    }
  };
}

describe("The GitHub helpers", function () {
  it("getRestClient is a function", function () {
    assert.strictEqual(typeof getRestClient, "function");
  });

  describe("storeResultsWithClient", function () {
    it("creates a new file when none exists (404)", async function () {
      const err404 = Object.assign(new Error("Not Found"), {status: 404});
      let capturedArgs;
      const mockOctokit = {
        repos: {
          getContent: async () => { throw err404; },
          createOrUpdateFileContents: async (args) => { capturedArgs = args; return {}; }
        }
      };

      await storeResultsWithClient(mockOctokit, SAMPLE_ENTRIES);

      assert.ok(capturedArgs, "createOrUpdateFileContents should have been called");
      assert.strictEqual(capturedArgs.sha, undefined);
      const decoded = Buffer.from(capturedArgs.content, "base64").toString("utf8");
      const line = JSON.parse(decoded.trim());
      assert.strictEqual(line.name, "foo-plugin:weekly");
      assert.ok(line.recorded_at);
    });

    it("appends to existing file content", async function () {
      const existing = JSON.stringify({name: "existing-plugin:weekly"}) + "\n";
      let capturedArgs;
      const mockOctokit = {
        repos: {
          getContent: async () => ({
            data: {
              sha: "existingsha123",
              content: Buffer.from(existing).toString("base64")
            }
          }),
          createOrUpdateFileContents: async (args) => { capturedArgs = args; return {}; }
        }
      };

      await storeResultsWithClient(mockOctokit, SAMPLE_ENTRIES);

      assert.ok(capturedArgs);
      assert.strictEqual(capturedArgs.sha, "existingsha123");
      const decoded = Buffer.from(capturedArgs.content, "base64").toString("utf8");
      const lines = decoded.trim().split("\n");
      assert.strictEqual(lines.length, 2);
      assert.strictEqual(JSON.parse(lines[0]).name, "existing-plugin:weekly");
      assert.strictEqual(JSON.parse(lines[1]).name, "foo-plugin:weekly");
    });

    it("propagates non-404 errors from getContent", async function () {
      const err403 = Object.assign(new Error("Forbidden"), {status: 403});
      const mockOctokit = {
        repos: {
          getContent: async () => { throw err403; },
          createOrUpdateFileContents: async () => {}
        }
      };
      await assert.rejects(() => storeResultsWithClient(mockOctokit, SAMPLE_ENTRIES), {status: 403});
    });
  });
});
