export const acceptedType: string[] = ["vless", "vmess", "trojan", "ssr"];
export const excludedType: string[] = ["https", "http"];
export const pattern: RegExp = new RegExp(`(?!${excludedType.join("|")})(${acceptedType.join("|")}):\/\/.+`, "gim");
export const path: string = process.cwd();

import { subMerge } from "./modules/sub_merge.js";
import { readFileSync, writeFileSync } from "fs";
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
  private connectCount: number = 0;
  private maxConcurrentTest: number = 50;
  private maxResult: number = 500;
  private numberOfTest: number = 2;
  private fishermanObjects: FishermanType[] = [];

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
      this.maxConcurrentTest = Math.round((configUrls.length / 100) / 2);
      if (this.maxConcurrentTest < 50) this.maxConcurrentTest = 50;

      logger.log(LogLevel.info, `Start test number: ${i}`);
      logger.log(LogLevel.info, `Max concurrent test: ${this.maxConcurrentTest}`);
      for (const configUrl of configUrls) {
        const modes = ["cdn", "sni"];
        if (configUrl.startsWith("ssr")) modes.shift();

        for (const mode of modes) {
          new Promise(async (resolve) => {
            const account = new Fisherman(configUrl);

            // account.config.tls = "tls";
            // account.config.skipCertVerify = true;
            // if (account.config.port == 80) {
            //   account.config.port = 443;
            // }

            await connect
              .connect(account.toSingBox(true, mode as "cdn" | "sni"))
              .then(async (res) => {
                if (res.error) {
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

                if (i == this.numberOfTest) {
                  await account.save().then((code) => {
                    this.configUrls.push(configUrl);
                    if (code != 2) {
                      this.configUrls.push(configUrl);
                      this.fishermanObjects.push(account.fisherman);
                      this.connectCount++;
                    }
                  });
                } else {
                  this.configUrls.push(configUrl);
                  logger.log(LogLevel.success, `[${account.config.vpn}] ${account.config.remark}: OK`);
                  this.connectCount++;
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
    }

    logger.log(LogLevel.info, "Sending sample to telegram channel ...");
    await bot.sendToChannel(
      this.fishermanObjects[Math.floor(Math.random() * this.fishermanObjects.length)],
      this.connectCount
    );
    writeFileSync("./result/nodes", this.configUrls.join("\n"));
  }
}

(async () => {
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
