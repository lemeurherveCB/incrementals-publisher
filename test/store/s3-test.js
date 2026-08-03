import assert from "assert";
import {storeWithClient} from "../../lib/store/s3.js";

const RAW = "name=foo-plugin:weekly;failCount=0;passCount=8;totalCount=8;duration=15.298;elapsed=39.122\n";

function makeMockS3() {
  const calls = [];
  return {
    send: async (cmd) => { calls.push(cmd.input); },
    calls,
  };
}

describe("S3 store backend", function () {
  describe("storeWithClient", function () {
    it("writes both the build file and latest.txt", async function () {
      const s3 = makeMockS3();

      await storeWithClient(s3, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(s3.calls.length, 2);
      assert.ok(s3.calls.some(c => c.Key.endsWith("21.txt")));
      assert.ok(s3.calls.some(c => c.Key.endsWith("latest.txt")));
    });

    it("stores identical content in both files", async function () {
      const s3 = makeMockS3();

      await storeWithClient(s3, "Plugins/bom/PR-1", "21", RAW);

      for (const c of s3.calls) {
        assert.strictEqual(c.Body, RAW);
      }
    });

    it("uses the correct S3 key paths", async function () {
      const s3 = makeMockS3();

      await storeWithClient(s3, "Plugins/bom/PR-1", "21", RAW);

      assert.strictEqual(s3.calls[0].Key, "Plugins/bom/PR-1/21.txt");
      assert.strictEqual(s3.calls[1].Key, "Plugins/bom/PR-1/latest.txt");
    });

    it("sets ContentType to text/plain", async function () {
      const s3 = makeMockS3();

      await storeWithClient(s3, "Plugins/bom/PR-1", "21", RAW);

      for (const c of s3.calls) {
        assert.strictEqual(c.ContentType, "text/plain");
      }
    });
  });
});
