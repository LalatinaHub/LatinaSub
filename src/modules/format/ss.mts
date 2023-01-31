import { db } from "../db.mjs";
import { logger, LogLevel } from "../logger.mjs";
import { V2Object } from "../../utils/types.mjs";
import { ssParser } from "../helper.mjs";
import { SSInterface } from "../../utils/types.mjs";

class SS {
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
        `CREATE TABLE IF NOT EXISTS SS (
                ID INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
                PASSWORD VARCHAR NOT NULL,
                ADDRESS VARCHAR NOT NULL,
                PORT INTEGER NOT NULL ON CONFLICT REPLACE DEFAULT 443,
                METHOD CARCHAR,
                PLUGIN VARCHAR,
                PATH VARCHAR,
                HOST VARCHAR,
                TLS VARCHAR,
                REMARK VARCHAR,
                CC VARCHAR,
                REGION VARCHAR,
                VPN VARCHAR NOT NULL ON CONFLICT REPLACE DEFAULT 'ss'
            );`,
        (err: Error) => {
          if (err) {
            logger.log(LogLevel.error, `[${this.account.vpn}] ${err.message}`);
            reject();
          } else {
            logger.log(LogLevel.success, "[DB] SS table successfully created!");
            resolve(0);
          }
        }
      );
    }).finally(() => {
      db.close(conn);
    });
  }

  async isExists() {
    const { address, port, id, plugin } = this.account;
    const conn = await db.connect();

    let query = `SELECT * FROM SS WHERE ADDRESS='${address}' AND PORT=${port} AND PASSWORD='${id}'`;
    if (plugin) query += ` AND PLUGIN="${plugin}"`;

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

    let query = `INSERT INTO SS (
        PASSWORD,
        ADDRESS,
        PORT,
        METHOD,
        PLUGIN,
        PATH,
        HOST,
        TLS,
        REMARK,
        CC,
        REGION
    ) VALUES (
        '${this.account.id}',
        '${this.account.address}',
        ${this.account.port},
        '${this.account.method}',
        '${this.account.plugin}',
        '${this.account.path}',
        '${this.account.host}',
        '${this.account.tls}',
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
    const query = "DROP TABLE IF EXISTS SS";

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
      let config = ssParser(this.url) as SSInterface;

      return {
        vpn: "ss",
        address: config.hostname,
        port: parseInt(`${config.port}`),
        plugin: config.query.plugin,
        method: config.query.method || "aes-256-gcm",
        id: config.query.password,
        path: config.query.path || "",
        host: config.query["obfs-host"] || config.query.host || "",
        tls: config.query.tls !== undefined ? "tls" : config.query.obfs === "tls" ? "tls" : "",
        remark: config.hash?.replace(/^#/, "") || config.hostname,
      } as V2Object;
    } catch (e: any) {
      return { vpn: "ss", error: e.message } as V2Object;
    }
  }

  toSingBox(account?: V2Object) {
    if (!account) account = this.account;

    return {
      type: "shadowsocks",
      tag: account.remark,

      server: account.address,
      server_port: account.port,
      method: account.method,
      password: account.id,
      plugin: account.plugin || "",
      plugin_opts: account.pluginParam || "",
    };
  }
}

export { SS };
