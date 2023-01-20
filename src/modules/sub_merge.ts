import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from "fs";
import { Sub } from "../utils/types.mjs";
import fetch from "node-fetch";
import { logger, LogLevel } from "./logger.mjs";

const subUrlList = [
  "https://raw.githubusercontent.com/LalatinaHub/Mineral/master/result/sub.json",
  "https://raw.githubusercontent.com/mahdibland/ShadowsocksAggregator/master/sub/sub_list.json",
  "https://raw.githubusercontent.com/paimonhub/Paimonnode/main/sublist.txt",
];

class SubMerge {
  private subFileList: string[] = [];
  private finalSub: Sub[] = [];

  async merge() {
    await this.getSubList();
    this.subFileList = readdirSync("./sub");
    for (const subName of this.subFileList) {
      const subFile: Sub[] = JSON.parse(readFileSync(`./sub/${subName}`).toString());

      for (const sub of subFile) {
        let isNew = false;
        if (this.finalSub.length) {
          for (const i in this.finalSub) {
            if (this.finalSub[i].remarks == sub.remarks) {
              let savedUrl = this.finalSub[i].url.split("|");
              for (const i in savedUrl) {
                savedUrl[i] = savedUrl[i].replace(/\/$/, "").replace("http://0.0.0.0:3333/get-base64?content=", "");
              }

              for (let url of sub.url.split("|")) {
                url = url.replace(/\/$/, "").replace("http://0.0.0.0:3333/get-base64?content=", "");
                if (!savedUrl.includes(url)) {
                  this.finalSub[i].url += `|${url}`;
                }
              }
              break;
            } else if (parseInt(i) + 1 >= this.finalSub.length) {
              isNew = true;
            }
          }
        } else {
          isNew = true;
        }

        if (isNew) {
          let url: string[] = [];
          for (let i of sub.url.split("|")) {
            url.push(i.replace(/\/$/, "").replace("http://0.0.0.0:3333/get-base64?content=", ""));
          }
          this.finalSub.push({
            ...sub,
            url: url.join("|"),
            id: this.finalSub.length,
          });
        }
      }
    }

    this.write();
  }

  private async getSubList() {
    if (!existsSync("./sub")) mkdirSync("./sub");
    for (const i in subUrlList) {
      const url = subUrlList[i];
      const controller = new globalThis.AbortController();
      const timeout = setTimeout(() => {
        controller.abort();
      }, 5000);

      try {
        const res = await fetch(url, {
          signal: controller.signal,
        });

        if (res.status != 200) {
          logger.log(LogLevel.error, `${res.status} ${res.statusText}`);
          continue;
        }

        let text = await res.text();
        let result: object[];
        try {
          result = JSON.parse(text);
        } catch (e: any) {
          result = [
            {
              id: Math.floor(Math.random() * 100),
              remarks: "Known Source",
              site: url,
              url: text.split("\n").join("|"),
              update_method: "auto",
              enabled: true,
            },
          ];
        }

        writeFileSync(`./sub/${parseInt(i) + 1}.json`, JSON.stringify(result, null, 2));
      } catch (e: any) {
        logger.log(LogLevel.error, e.message);
      }

      clearTimeout(timeout);
    }
  }

  private write() {
    writeFileSync("./sub_list.json", JSON.stringify(this.finalSub, null, 2));
  }
}

const subMerge = new SubMerge();
export { subMerge };
