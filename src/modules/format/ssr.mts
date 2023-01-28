import { db } from "../db.mjs";
import { logger, LogLevel } from "../logger.mjs";
import { V2Object } from "../../utils/types.mjs";
import { ssrParser } from "../helper.mjs";
import { SSRInterface } from "../../utils/types.mjs";

class SSR {
  account: V2Object;
  url: string;

  constructor(url: string) {
    this.url = url;
    this.account = this.toV2Object();
  }

  async initDb() {
    const conn = await db.connect();

    return await new Promise((resolve, reject) => {
      conn.run(
        `CREATE TABLE IF NOT EXISTS SSR (
                ID INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
                PASSWORD VARCHAR NOT NULL,
                ADDRESS VARCHAR NOT NULL,
                PORT INTEGER NOT NULL ON CONFLICT REPLACE DEFAULT 443,
                METHOD CARCHAR,
                PROTOCOL VARCHAR,
                PROTOCOL_PARAM VARCHAR,
                OBFS VARCHAR,
                OBFS_PARAM VARCHAR,
                [GROUP] VARCHAR,
                REMARK VARCHAR,
                CC VARCHAR,
                REGION VARCHAR,
                VPN VARCHAR NOT NULL ON CONFLICT REPLACE DEFAULT 'ssr'
            );`,
        (err: Error) => {
          if (err) {
            logger.log(LogLevel.error, `[${this.account.vpn}] ${err.message}`);
            reject();
          } else {
            logger.log(LogLevel.success, "[DB] SSR table successfully created!");
            resolve(0);
          }
        }
      );
    }).finally(() => {
      db.close(conn);
    });
  }

  async isExists() {
    const { address, port, id } = this.account;
    const conn = await db.connect();

    const query = `SELECT * FROM SSR WHERE ADDRESS='${address}' AND PORT=${port} AND PASSWORD='${id}'`;

    return await new Promise((resolve, reject) => {
      conn.get(query, (err: Error, row: any) => {
        if (err) {
          logger.log(LogLevel.error, `[${this.account.vpn}] ${err.message}`);
          reject();
        }

        resolve(row?.["ID"]);
      });
    }).finally(() => {
      db.close(conn);
    });
  }

  async insert() {
    const isExists = await this.isExists();
    const conn = await db.connect();
    if (isExists) {
      db.close(conn);
      logger.log(LogLevel.info, `[${this.account.vpn}] ${this.account.remark} is exists on database!`);
      return 2;
    }

    let query = `INSERT INTO SSR (
        PASSWORD,
        ADDRESS,
        PORT,
        METHOD,
        PROTOCOL,
        PROTOCOL_PARAM,
        OBFS,
        OBFS_PARAM,
        [GROUP],
        REMARK,
        CC,
        REGION
    ) VALUES (
        '${this.account.id}',
        '${this.account.address}',
        ${this.account.port},
        '${this.account.method}',
        '${this.account.proto}',
        '${this.account.protoParam}',
        '${this.account.obfs}',
        '${this.account.obfsParam}',
        '${this.account.group}',
        '${this.account.remark}',
        '${this.account.cc}',
        '${this.account.region}'
    );`;

    return await new Promise((resolve, reject) => {
      conn.run(query, (err: Error) => {
        if (err) {
          logger.log(LogLevel.error, `[${this.account.vpn}] ${err.message}`);
          reject(err.name);
        }

        logger.log(LogLevel.success, `[${this.account.vpn}] ${this.account.remark} successfully added!`);
        resolve(0);
      });
    }).then(() => {
      db.close(conn);
    });
  }

  async drop() {
    const conn = await db.connect();
    const query = "DROP TABLE IF EXISTS SSR";

    await new Promise((resolve, reject) => {
      conn.run(query, (err: Error) => {
        if (err) {
          logger.log(LogLevel.error, `[${this.account.vpn}] ${err.message}`);
          reject();
        }

        resolve(0);
      });
    });
  }

  // Convert to various format
  toV2Object(): any {
    try {
      let config = ssrParser(this.url) as SSRInterface;

      return {
        vpn: "ssr",
        address: config.hostname,
        port: parseInt(`${config.port}`),
        obfs: config.query.obfs?.match(/(plain|random)/) ? "http_simple" : config.query.obfs,
        obfsParam: config.query.obfsparam || "",
        proto: config.query.protocol || "origin",
        protoParam: config.query.protoparam || "",
        method: config.query.method || "aes-256-cfb",
        id: config.query.password,
        remark: config.query.remarks,
        group: config.query.group,
        tls: config.query.obfs?.match("obfs") ? "tls" : "",
      } as V2Object;
    } catch (e: any) {
      return { vpn: "ssr", error: e.message } as V2Object;
    }
  }

  toSingBox(account?: V2Object) {
    if (!account) account = this.account;

    return {
      type: "shadowsocksr",
      tag: account.remark,

      server: account.address,
      server_port: account.port,
      method: account.method,
      password: account.id,
      obfs: account.obfs,
      obfs_param: account.obfsParam,
      protocol: account.proto,
      protocol_param: account.protoParam,
    };
  }
}

export { SSR };
