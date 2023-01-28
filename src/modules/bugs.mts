import { readFileSync } from "fs";
import { V2Object } from "../utils/types.mjs";

interface BugsObject {
  sni: Array<string>;
  cdn: Array<string>;
}

class Bugs {
  private _sni: Array<string> = [];
  private _cdn: Array<string> = [];
  private bugs: BugsObject;

  constructor() {
    this.bugs = JSON.parse(readFileSync(`./resources/bug.json`).toString());

    this._sni = this.bugs.sni;
    this._cdn = this.bugs.cdn;
  }

  get sni(): string {
    return this._sni[Math.floor(Math.random() * this._sni.length)];
  }

  get cdn(): string {
    return this._cdn[Math.floor(Math.random() * this._cdn.length)];
  }

  fill(account: V2Object, mode: "sni" | "cdn") {
    const sni = this.sni;
    const cdn = this.cdn;

    if (account.vpn == "ssr") {
      if (account.obfs?.match("tls")) {
        account.obfsParam = `obfs=tls;obfs-host=${sni}`;
      } else {
        account.obfsParam = `obfs=http;obfs-host=${sni}`;
      }
    } else if (account.vpn == "ss") {
      if (account.plugin?.match("obfs")) {
        account.pluginParam = `obfs=${account.tls ? "tls" : "http"};obfs-host=${sni}`;
      } else if (account.plugin?.match("v2ray")) {
        if (mode == "cdn") {
          account.pluginParam = `path=${account.path};host=${account.address}${account.tls ? ";tls" : ""}`;
          account.address = cdn;
        } else {
          account.pluginParam = `path=${account.path};host=${sni}${account.tls ? ";tls" : ""}`;
        }
      }
    } else {
      if (mode == "cdn") {
        account.address = cdn;
      } else {
        account.host = sni;
        account.sni = sni;
      }
    }
    return account;
  }
}

export { Bugs };
