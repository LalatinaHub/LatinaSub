import { spawn } from "child_process";
import { ConnectServer, Country } from "../utils/types.mjs";
import { sleep } from "./helper.mjs";
import { SocksProxyAgent } from "socks-proxy-agent";
import fetch from "node-fetch";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "fs";
import { path } from "../index.js";
import { isIPv4 } from "is-ip";

class Connect {
  connectionNumber = 1;
  countries: Country[] = JSON.parse(readFileSync("./countries.json").toString());

  private async _connect(config: string, port: number): Promise<ConnectServer> {
    let error: boolean = true;
    let cc: string = "";
    let cn: string = "";
    let ip: string = "";
    let region: string = "";
    let server: string = "";
    let message: string = "Unknown Error!";

    const singBox = spawn("./bin/sing-box", ["run", "-c", `${config}`]);

    singBox.stderr.on("data", (res: any) => {
      const message = res.toString();
      // console.log(res.toString());
      if (message.match(/error:.+/)) {
        error = message.match(/error:(.+)/)[1];
      }
    });

    await sleep(1000);

    try {
      await fetch("http://ipapi.co/json", {
        agent: new SocksProxyAgent(
          {
            hostname: "127.0.0.1",
            port: port,
            protocol: "socks",
            tls: {
              rejectUnauthorized: false,
            },
          },
          {
            timeout: 30000,
          }
        ),
      }).then(async (res) => {
        if (res.status == 200) {
          const data = JSON.parse(await res.text());
          if (data.error) {
            message = data.error;
          } else {
            error = false;
            message = "";

            cc = data.country_code || "XX"; // Default country code
            cn = data.country_name || "Other"; // Default country name
            ip = "";
            region = "World";
            server = "Unknown";

            for (const country of this.countries) {
              if (cc == "XX") {
                region = "World";
                break;
              } else if (country.code == cc) {
                region = country.region;
                break;
              }
            }

            if (data.org) {
              // server = data.org.match(/^(a-zA-Z\d)+/)?.[1];
              server = data.org;
              server = server.replace(/,/m, " ");
            }

            if (data.ip) {
              if (isIPv4(data.ip)) ip = data.ip;
            }
          }
        }
      });
    } catch (e: any) {
      // console.log(e.message);
      if ((e.message as string).match(/(aborted|socket hang up)/)) {
        message = "Timeout!";
      } else {
        message = e.message;
      }
    }

    await new Promise((resolve) => {
      singBox.kill();

      singBox.on("close", () => {
        resolve(0);
      });
    });

    return {
      error,
      cc,
      cn,
      ip,
      region,
      server,
      message,
    };
  }

  async connect(account: any): Promise<ConnectServer> {
    const port = this.calculatePort();
    if (!existsSync("./resources/config-test")) mkdirSync("./resources/config-test");
    const savePath = `${path}/resources/config-test/test-${port}.json`;
    let boxConfig = JSON.parse(readFileSync("./resources/config.json").toString());

    boxConfig.inbounds[0].listen_port = port;
    boxConfig.outbounds.push(account);
    boxConfig.outbounds[3].outbounds.push(account.tag);

    writeFileSync(savePath, JSON.stringify(boxConfig, null, 2));

    this.connectionNumber++;
    if (this.connectionNumber > 2000) {
      this.connectionNumber = 1;
    }
    return await this._connect(savePath, port);
  }

  calculatePort(): number {
    return this.connectionNumber + 10000;
  }
}

const connect = new Connect();
export { connect, Connect };
