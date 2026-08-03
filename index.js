import {readFile} from "fs/promises";
import {timingSafeEqual} from "crypto";
import express from "express";
import helmet from "helmet";
import config from "./lib/config.js";
import {store} from "./lib/store/azure.js";

const packageJson = JSON.parse(await readFile(new URL("./package.json", import.meta.url)));

const app = express();
const port = config.PORT;

app.use(helmet());
app.use(express.urlencoded({extended: false}));
app.use(express.json());

app.get("/readiness", (_req, res) => {
  res.status(200).json({status: "OK"});
});

app.get("/liveness", (_req, res) => {
  res.status(200).json({status: "OK", version: packageJson.version});
});

const presharedKey = Buffer.from(config.PRESHARED_KEY);

function checkAuth(req, res) {
  const token = (req.get("Authorization") || "").replace(/^Bearer /, "");
  const tokenBuf = Buffer.from(token);
  if (tokenBuf.length !== presharedKey.length || !timingSafeEqual(tokenBuf, presharedKey)) {
    res.status(403).send("Not authorized");
    return false;
  }
  return true;
}

app.post("/bom-results", async (req, res, next) => {
  try {
    if (!checkAuth(req, res)) return;

    const {job_name: jobName, build_id: buildId, results} = req.body;
    if (!jobName || !buildId || !results || typeof results !== "string") {
      res.status(400).send("Missing required fields: job_name, build_id, results");
      return;
    }

    console.log(`Storing results for ${jobName} build ${buildId}`);
    await store(jobName, buildId, results);
    res.status(200).send("OK");
  } catch (err) {
    next(err);
  }
});

app.use((err, _req, res, _next) => {
  console.error(err.stack);
  res.status(err.status || 400).send(err.message || "Unknown error");
});

process.on("SIGINT", () => process.exit());

app.listen(port, () => {
  console.log(`BOM results publisher listening at http://localhost:${port}`);
});
