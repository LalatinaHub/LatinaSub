import { Vless } from "./format/vless.mjs";
import { Vmess } from "./format/vmess.mjs";
import { Trojan } from "./format/trojan.mjs";
import { SSR } from "./format/ssr.mjs";
import { SS } from "./format/ss.mjs";
import { Bugs } from "./bugs.mjs";
import { V2Object } from "../utils/types.mjs";

export type FishermanType = Vless | Vmess | Trojan | SSR;

class Fisherman {
  private _fisherman!: FishermanType;

  constructor(url: string) {
    if (url.startsWith("vmess")) {
      this._fisherman = new Vmess(url);
    } else if (url.startsWith("trojan")) {
      this._fisherman = new Trojan(url);
    } else if (url.startsWith("ssr")) {
      this._fisherman = new SSR(url);
    } else if (url.startsWith("ss")) {
      this._fisherman = new SS(url);
    } else if (url.startsWith("vless")) {
      this._fisherman = new Vless(url);
    }
  }

  get config() {
    return this._fisherman.account;
  }

  set config(account: V2Object) {
    this._fisherman.account = account;
  }

  get fisherman(): FishermanType {
    return this._fisherman;
  }

  fillBugs(mode: "cdn" | "sni" = "sni") {
    const bugs = new Bugs();
    const account = this.toV2Object();
    return bugs.fill(account, mode);
  }

  initDb() {
    return this._fisherman.initDb();
  }

  toV2Object(): V2Object {
    return this._fisherman.toV2Object();
  }

  toSingBox(withBugs: boolean = false, mode: "cdn" | "sni" = "sni") {
    if (withBugs) {
      return this._fisherman.toSingBox(this.fillBugs(mode));
    } else {
      return this._fisherman.toSingBox();
    }
  }

  async save() {
    return this._fisherman.insert();
  }

  async drop() {
    this._fisherman.drop();
  }
}

export { Fisherman };
