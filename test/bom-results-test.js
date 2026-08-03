import assert from "assert";
import {parseResults} from "../lib/bom-results.js";

const SINGLE_LINE = "name=aws-credentials-plugin:weekly;failCount=0;skipCount=0;passCount=8;totalCount=8;duration=15.298;elapsed=39.122;plugins=[aws-credentials];pluginCount=1;attempt=1;build_id=21;job_base_name=PR-7078;short_commit_id=d73ef89";
const MULTI_PLUGIN_LINE = "name=pipeline-maven-plugin:weekly;failCount=0;skipCount=4;passCount=268;totalCount=272;duration=121.903;elapsed=253.155;plugins=[pipeline-maven, pipeline-maven-api, pipeline-maven-database];pluginCount=3;attempt=1;build_id=21;job_base_name=PR-7078;short_commit_id=d73ef89";

describe("parseResults", function () {
  it("returns empty array for empty input", function () {
    assert.deepStrictEqual(parseResults(""), []);
    assert.deepStrictEqual(parseResults(null), []);
    assert.deepStrictEqual(parseResults("   \n\n  "), []);
  });

  it("parses a single line into one object", function () {
    const results = parseResults(SINGLE_LINE);
    assert.strictEqual(results.length, 1);
  });

  it("parses multi-line input into multiple objects", function () {
    const results = parseResults(SINGLE_LINE + "\n" + MULTI_PLUGIN_LINE);
    assert.strictEqual(results.length, 2);
    assert.strictEqual(results[0].name, "aws-credentials-plugin:weekly");
    assert.strictEqual(results[1].name, "pipeline-maven-plugin:weekly");
  });

  it("coerces integer fields", function () {
    const [r] = parseResults(SINGLE_LINE);
    assert.strictEqual(r.failCount, 0);
    assert.strictEqual(r.skipCount, 0);
    assert.strictEqual(r.passCount, 8);
    assert.strictEqual(r.totalCount, 8);
    assert.strictEqual(r.pluginCount, 1);
    assert.strictEqual(r.attempt, 1);
    assert.strictEqual(r.build_id, 21);
    assert.strictEqual(typeof r.failCount, "number");
  });

  it("coerces float fields", function () {
    const [r] = parseResults(SINGLE_LINE);
    assert.strictEqual(r.duration, 15.298);
    assert.strictEqual(r.elapsed, 39.122);
    assert.strictEqual(typeof r.duration, "number");
  });

  it("parses single plugin as array of one", function () {
    const [r] = parseResults(SINGLE_LINE);
    assert.deepStrictEqual(r.plugins, ["aws-credentials"]);
  });

  it("parses multiple plugins as array", function () {
    const [r] = parseResults(MULTI_PLUGIN_LINE);
    assert.deepStrictEqual(r.plugins, ["pipeline-maven", "pipeline-maven-api", "pipeline-maven-database"]);
  });

  it("preserves string fields", function () {
    const [r] = parseResults(SINGLE_LINE);
    assert.strictEqual(r.job_base_name, "PR-7078");
    assert.strictEqual(r.short_commit_id, "d73ef89");
  });

  it("ignores blank lines between entries", function () {
    const results = parseResults(SINGLE_LINE + "\n\n" + MULTI_PLUGIN_LINE + "\n");
    assert.strictEqual(results.length, 2);
  });
});
