import assert from "assert";
import {getRestClient} from "../lib/github.js";

describe("The GitHub helpers", function () {
  it("getRestClient is a function", function () {
    assert.strictEqual(typeof getRestClient, "function");
  });
});
