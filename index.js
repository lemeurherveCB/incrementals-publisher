import {readFile} from "fs/promises";
import bcrypt from "bcrypt";
import express from "express";
import winston from "winston";
import expressWinston from "express-winston";
import bodyParser from "body-parser";
import helmet from "helmet";
import asyncWrap from "express-async-wrap";
import config from "./lib/config.js";

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
