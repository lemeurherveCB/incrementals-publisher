import {readFile} from "fs/promises";
import bcrypt from "bcrypt";
import express from "express";
import winston from "winston";
import expressWinston from "express-winston";
import bodyParser from "body-parser";
import helmet from "helmet";
import asyncWrap from "express-async-wrap";
import config from "./lib/config.js";
import {parseResults} from "./lib/bom-results.js";
import {storeResults} from "./lib/github.js";

const packageJson = JSON.parse(await readFile(new URL("./package.json", import.meta.url)));

const app = express();
const port = config.PORT;

const logger = winston.createLogger({
  level: "debug",
  transports: [
    new winston.transports.Console({})
  ],
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.align(),
    winston.format.splat(),
    winston.format.printf(info => `${info.timestamp} ${info.level}: ${info.message}`)
  ),
  exitOnError: false,
});

app.use(expressWinston.logger({
  winstonInstance: logger,
}));

app.use(helmet());
app.use(bodyParser.urlencoded({extended: false}));
app.use(bodyParser.json());

app.get("/readiness", asyncWrap(async (_req, res) => {
  res.status(200).json({status: "OK"});
}));

app.get("/liveness", asyncWrap(async (_req, res) => {
  res.status(200).json({
    status: "OK",
    version: packageJson.version
  });
}));

const encodedPassword = bcrypt.hashSync(config.PRESHARED_KEY, 10);

async function checkAuth(req, res) {
  const token = (req.get("Authorization") || "").replace(/^Bearer /, "");
  const ok = await bcrypt.compare(token, encodedPassword);
  if (!ok) {
    res.status(403).send("Not authorized");
    return false;
  }
  return true;
}

app.post("/bom-results", asyncWrap(async (req, res) => {
  if (!await checkAuth(req, res)) return;

  const raw = req.body.results;
  if (!raw || typeof raw !== "string") {
    res.status(400).send("Missing or invalid 'results' field");
    return;
  }

  const entries = parseResults(raw);
  if (entries.length === 0) {
    res.status(400).send("No results found in payload");
    return;
  }

  logger.info("Storing %d BOM result(s)", entries.length);
  await storeResults(entries);
  res.status(200).json({stored: entries.length});
}));

/*Error handler goes last */
app.use(function (err, req, res, next) {
  logger.error(err.stack);
  res.status(err.status || err.code || 400).send(err.message || "Unknown error");
  next();
});

process.on("SIGINT", () => {
  logger.info("Got SIGINT");
  process.exit();
});

app.listen(port, () => {
  logger.info(`BOM results publisher listening at http://localhost:${port}`);
});

export {app, logger, encodedPassword};
