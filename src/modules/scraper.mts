import { base64Decode, sleep, isRunning, urlParser } from "./helper.mjs";
import { acceptedType, pattern } from "../index.js";
import { logger, LogLevel } from "./logger.mjs";
import { Fisherman } from "./fisherman.mjs";
import { subconverter } from "./subconverter.mjs";
import fetch from "node-fetch";
import { readFileSync, writeFileSync } from "fs";

type ScraperType = {
  error?: boolean;
  result: string[];
};

class Scraper {
  private blacklistSub: string[] = readFileSync("./result/blacklist_sub").toString().split("\n");

  private async get(url: string, target: string): Promise<string[]> {
    let configs: string[] = [];

    if ((await isRunning("subconverter")) <= 0) {
      while ((await isRunning("subconverter")) <= 0) {
        subconverter.start();
        await sleep(2000);
      }
    }

    const controller = new globalThis.AbortController();
    const timeout = setTimeout(() => {
      controller.abort();
    }, 5000);

    try {
      let res = await fetch(url, {
        method: "GET",
        follow: 10,
        signal: controller.signal,
      });

      if (res.status != 200) {
        logger.log(LogLevel.error, `${url} -> ${res.status} ${res.statusText}`);

        if (!this.blacklistSub.includes(url)) this.blacklistSub.push(url);
        return [];
      } else {
        clearTimeout(timeout);
      }

      const parsedUrl = urlParser(res.url);
      if (parsedUrl.query.url && parsedUrl.query.target) {
        url = parsedUrl.query.url as string;
      }
      let repoText = await res.text();

      configs = repoText.match(pattern) || [];
      if (configs.length <= 0) {
        configs = base64Decode(repoText).match(pattern) || [];
        if (configs.length <= 0) {
          res = await fetch(`http://127.0.0.1:25500/sub?target=${target == "vmess" ? "v2ray" : target}&url=${url}`);
          let subText = await res.text();

          configs = base64Decode(subText).match(pattern) || [];
          if (configs.length <= 0) {
            configs = JSON.parse(repoText);
          }
        }
      }
    } catch (e: any) {
      logger.log(LogLevel.error, `[${target}] ${url} -> ${e.message}`);
    } finally {
      try {
        configs = configs.join("\n").match(pattern) || [];
      } catch (e: any) {
        logger.log(LogLevel.error, `[${target}] ${url} -> ${e.message}`);
        configs = [];
      }
    }

    if (configs.length >= 1) {
      logger.log(
        LogLevel.success,
        `[${target}] ${url} -> found ${configs.join("|").match(new RegExp(target, "gim"))?.length || 0} !`
      );
    }
    return configs;
  }

  async start(urls: string[]): Promise<ScraperType> {
    const isExists: string[] = [];
    const result: string[] = [];
    const configs: string[] = [];

    let onScrape: number[] = [];
    for (const url of urls) {
      // Blacklist filter
      if (this.blacklistSub.includes(url)) continue;

      new Promise(async (resolve) => {
        for (const target of acceptedType) {
          onScrape.push(1);
          await this.get(url, target)
            .then((res) => {
              res.forEach((configUrl) => {
                const { address, host, port, id, path, serviceName, network, vpn } = new Fisherman(
                  configUrl
                ).toV2Object();
                const uniqueId = `${address}_${port}_${id}_${host}_${path}_${network}_${serviceName}_${vpn}`;

                if (!isExists.includes(uniqueId)) {
                  result.push(configUrl);
                  isExists.push(uniqueId);
                }
              });
            })
            .finally(() => {
              onScrape.shift();
            });
        }

        resolve(0);
      });

      if (onScrape.length >= 20) {
        await sleep(1000);
      }
    }

    while (onScrape[0]) {
      await sleep(1000);
    }

    for (const config of configs) {
      if (!config) continue;
      if (!acceptedType.includes((config.match(/(.+):\/\//) || [])[1])) continue;

      result.push(config);
    }

    writeFileSync("./result/blacklist_sub", this.blacklistSub.join("\n"));
    return {
      result,
    };
  }
}

const scraper = new Scraper();
export { scraper };
