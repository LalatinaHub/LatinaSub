export const acceptedType: string[] = ["vless", "vmess", "trojan", "ssr", "ss"];
export const excludedType: string[] = ["https", "http"];
export const pattern: RegExp = new RegExp(`(?!${excludedType.join("|")})(${acceptedType.join("|")}):\/\/.+`, "gim");
export const path: string = process.cwd();

import { subMerge } from "./modules/sub_merge.js";
import { copyFileSync, readFileSync, writeFileSync } from "fs";
import { scraper } from "./modules/scraper.mjs";
import { Sub } from "./utils/types.mjs";
import { Fisherman, FishermanType } from "./modules/fisherman.mjs";
import { connect } from "./modules/connect.mjs";
import { logger, LogLevel } from "./modules/logger.mjs";
import { isRunning, sleep } from "./modules/helper.mjs";
import { exec } from "child_process";
import { subconverter } from "./modules/subconverter.mjs";
import { db } from "./modules/db.mjs";
import { bot } from "./modules/tg.mjs";
import countryCodeEmoji from "country-code-emoji";

class Main {
  private urls: string[] = [];
  private configUrls: string[] = [];
  private blacklistNode: string[] = readFileSync("./result/blacklist_node").toString().split("\n");
  private connectCount: number = 0;
  private maxConcurrentTest: number = 50;
  private maxResult: number = 999999999;
  private numberOfTest: number = 2;
  private fishermanPool: FishermanType[] = [];

  constructor() {
    const subList: Sub[] = JSON.parse(readFileSync("./sub_list.json").toString());
    for (const sub of subList) {
      this.urls.push(...sub.url.split("|"));
    }
  }

  async scrape() {
    const configUrls = (await scraper.start(this.urls)).result;
    this.configUrls.push(...configUrls);
    console.log("");
    logger.log(LogLevel.info, `Total Found -> ${this.configUrls.length}`);
  }

  async connectTest() {
    for (let i = 1; i <= this.numberOfTest; i++) {
      const configUrls = structuredClone(this.configUrls);

      this.configUrls = [];
      this.connectCount = 0;
      this.maxConcurrentTest = Math.round(configUrls.length / 100 / 2);
      if (this.maxConcurrentTest > 100) this.maxConcurrentTest = 100;
      if (this.maxConcurrentTest < 50) this.maxConcurrentTest = 50;

      logger.log(LogLevel.info, `Start test number: ${i}`);
      logger.log(LogLevel.info, `Max concurrent test: ${this.maxConcurrentTest}`);
      for (const configUrl of configUrls) {
        const modes = ["cdn", "sni"];
        switch (configUrl.replace(/:.+/, "")) {
          case "ssr":
          case "ss":
            modes.shift();
        }

        for (const mode of modes) {
          new Promise(async (resolve) => {
            const account = new Fisherman(configUrl);

            // Blacklist filter
            const { address, host, port, id, path, serviceName, network, vpn } = account.toV2Object();
            const uniqueId = `${address}_${port}_${id}_${host}_${path}_${network}_${serviceName}_${mode}_${vpn}`;
            if (this.blacklistNode.includes(uniqueId)) {
              // logger.log(LogLevel.info, "Blacklisted node found !");
              resolve(0);
            }

            // Override config
            switch (configUrl.replace(/:.+/, "")) {
              case "ssr":
                account.config.network = "OBFS";
                break;
              case "ss":
                switch (account.config.port) {
                  case 443:
                  case 8443:
                    account.config.tls = "tls";
                    break;
                }

                switch (account.config.plugin) {
                  case "obfs-local":
                    account.config.network = "OBFS";
                    break;
                  case "v2ray-plugin":
                    account.config.network = "V2RAY";
                    break;
                }
                break;
            }

            account.config.skipCertVerify = true;
            account.config.network = account.config.network?.toLowerCase();
            switch (account.config.network) {
              case "ws":
                if (!account.config.path) account.config.path = "/";
                break;
              case "grpc":
              case "obfs":
              case "v2ray":
                break;
              default:
                account.config.network = "tcp";
            }

            await connect
              .connect(account.toSingBox(true, mode as "cdn" | "sni"))
              .then(async (res) => {
                if (res.error) {
                  if (!this.blacklistNode.includes(uniqueId)) this.blacklistNode.push(uniqueId);

                  // logger.log(LogLevel.error, `[${account.config.vpn}] ${account.config.remark} -> ${res.message}`);
                  return;
                }

                account.config.cc = res.cc;
                account.config.countryName = res.cn;
                account.config.region = res.region;
                account.config.server = res.server;
                account.config.cdn = mode == "cdn";
                account.config.remark = `${this.connectCount + 1} ${countryCodeEmoji(res.cc)} ${
                  res.server
                } ${account.config.network?.toUpperCase()} ${mode.toUpperCase()} ${
                  account.config.tls ? "TLS" : "NTLS"
                }`;

                this.configUrls.push(configUrl);
                logger.log(LogLevel.success, `[${account.config.vpn}] ${account.config.remark}: OK`);
                this.connectCount++;

                if (i == this.numberOfTest) {
                  this.fishermanPool.push(account.fisherman);
                }
              })
              .finally(() => {
                resolve(0);
              });
          });
        }

        let isStuck = 30;
        while ((await isRunning("sing-box")) > this.maxConcurrentTest) {
          await sleep(2000);
          isStuck--;

          if (!isStuck) {
            exec("pkill sing-box");
          }
        }

        if (this.connectCount > this.maxResult) break;
      }

      console.log("");
      logger.log(LogLevel.info, `Total Connected Account -> ${this.connectCount}`);
      logger.log(LogLevel.info, "Sleeping for 1 minute...");
      await sleep(60000);
    }

    logger.log(LogLevel.info, "Saving accounts to database...");
    this.connectCount = 0;
    for (const account of this.fishermanPool) {
      await account.insert().then((code) => {
        if (code != 2) this.connectCount++;
      });
    }

    logger.log(LogLevel.info, "Sending sample to telegram channel ...");
    await bot.sendToChannel(
      this.fishermanPool[Math.floor(Math.random() * this.fishermanPool.length)],
      this.connectCount
    );
    writeFileSync("./result/nodes", this.configUrls.join("\n"));
    writeFileSync("./result/blacklist_node", this.blacklistNode.join("\n"));

    // API Mode
    // The DB will be copied to ../resources (refer LatinaApi) too
    if (process.env.API_MODE) {
      copyFileSync("./result/db.sqlite", "../resources/db.sqlite");
    }
  }
}

(async () => {
  // Print log date
  logger.log(LogLevel.info, `Run Date => ${new Date()}`);

  // Initialize
  await db.make();
  for (const vpn of acceptedType) {
    const fisherman = new Fisherman(vpn);
    await fisherman.drop();
    await sleep(200);
    await fisherman.initDb();
  }

  await sleep(1000);

  // Merge sublist
  await subMerge.merge();

  subconverter.start();

  // Start
  const main = new Main();
  await main.scrape();

  logger.log(LogLevel.info, "Start the test ...");
  await main.connectTest();

  exec("pkill sing-box");
  exec("pkill subconverter");
})();
