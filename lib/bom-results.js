const INT_FIELDS = new Set(["failCount", "skipCount", "passCount", "totalCount", "pluginCount", "attempt", "build_id"]);
const FLOAT_FIELDS = new Set(["duration", "elapsed"]);

export function parseResults(text) {
  if (!text) return [];
  return text.split("\n")
    .map(line => line.trim())
    .filter(line => line.length > 0)
    .map(line => {
      const entry = {};
      for (const token of line.split(";")) {
        const eq = token.indexOf("=");
        if (eq === -1) continue;
        const key = token.slice(0, eq);
        const raw = token.slice(eq + 1);
        if (INT_FIELDS.has(key)) {
          entry[key] = parseInt(raw, 10);
        } else if (FLOAT_FIELDS.has(key)) {
          entry[key] = parseFloat(raw);
        } else if (key === "plugins") {
          entry[key] = raw.replace(/^\[|\]$/g, "").split(",").map(s => s.trim()).filter(s => s.length > 0);
        } else {
          entry[key] = raw;
        }
      }
      return entry;
    });
}
