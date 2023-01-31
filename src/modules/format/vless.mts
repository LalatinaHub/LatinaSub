import { db } from "../db.mjs";
import { logger, LogLevel } from "../logger.mjs";
import { V2Object } from "../../utils/types.mjs";
import { urlParser } from "../helper.mjs";

class Vless {
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
        `CREATE TABLE IF NOT EXISTS Vless (
                ID INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
                PASSWORD VARCHAR NOT NULL,
                ADDRESS VARCHAR NOT NULL,
                PORT INTEGER NOT NULL ON CONFLICT REPLACE DEFAULT 443,
                SECURITY VARCHAR NOT NULL ON CONFLICT REPLACE DEFAULT 'tls',
                FLOW VARCHAR,
                LEVEL INTEGER NOT NULL ON CONFLICT REPLACE DEFAULT 8,
                HOST VARCHAR,
                TYPE VARCHAR,
                PATH VARCHAR,
                SERVICE_NAME VARCHAR,
                MODE VARCHAR,
                ALLOW_INSECURE BOOLEAN NOT NULL ON CONFLICT REPLACE DEFAULT 1,
                SNI VARCHAR,
                REMARK VARCHAR,
                IS_CDN BOOLEAN,
                CC VARCHAR,
                REGION VARCHAR,
                VPN VARCHAR NOT NULL ON CONFLICT REPLACE DEFAULT 'vless'
            );`,
        (err: Error) => {
          if (err) {
            logger.log(LogLevel.error, `[${this.account.vpn}] ${err.message}`);
            reject();
          } else {
            logger.log(LogLevel.success, "[DB] Vless table successfully created!");
            resolve(0);
          }
        }
      );
    }).finally(() => {
      db.close(conn);
    });
  }

  async isExists() {
    const { address, host, network, port, id, path, cdn, serviceName, mode } = this.account;
    const conn = await db.connect();

    let query = `SELECT * FROM Vless WHERE PORT=${port} AND PASSWORD='${id}' AND IS_CDN=${
      cdn ? 1 : 0
    } AND TYPE='${network}'`;
    if (path) query += ` AND PATH='${path}'`;
    if (serviceName) query += ` AND SERVICE_NAME='${serviceName}'`;
    if (mode) query += ` AND MODE='${mode}'`;
    if (cdn) query += ` AND HOST='${host}'`;
    else query += ` AND ADDRESS='${address}'`;

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

    let query = `INSERT INTO Vless (
        PASSWORD,
        ADDRESS,
        PORT,
        SECURITY,
        FLOW,
        LEVEL,
        HOST,
        TYPE,
        PATH,
        SERVICE_NAME,
        MODE,
        ALLOW_INSECURE,
        SNI,
        REMARK,
        IS_CDN,
        CC,
        REGION
    ) VALUES (
        '${this.account.id}',
        '${this.account.address}',
        ${this.account.port},
        '${this.account.tls}',
        '${this.account.flow}',
        ${this.account.level},
        '${this.account.host}',
        '${this.account.network}',
        '${this.account.path}',
        '${this.account.serviceName}',
        '${this.account.mode}',
        ${this.account.skipCertVerify ? 1 : 0},
        '${this.account.sni}',
        '${this.account.remark}',
        ${this.account.cdn ? 1 : 0},
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
    const query = "DROP TABLE IF EXISTS Vless";

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
      let config = urlParser(this.url);

      return {
        vpn: "vless",
        address: config.hostname,
        port: parseInt(`${config.port}`),
        host: config.query.host || "",
        id: config.auth,
        network: config.query.type,
        path: config.query.path || "",
        serviceName: config.query.serviceName || "",
        mode: config.query.mode || "",
        skipCertVerify: true,
        tls: config.query.security ? "tls" : "",
        sni: config.query.sni || config.query.host || "",
        flow: config.query.flow || "",
        level: config.query.level || 8,
        remark: config.hash?.replace(/^#/, ""),
      } as V2Object;
    } catch (e: any) {
      return { vpn: "vless", error: e.message } as V2Object;
    }
  }

  toSingBox(account?: V2Object) {
    if (!account) account = this.account;

    let proxy: any = {
      type: account.vpn,
      tag: account.remark,
      server: account.address,
      server_port: account.port,
      uuid: account.id,
      tls: {
        enabled: account.tls ? true : false,
        insecure: account.skipCertVerify || true,
        server_name: account.sni || account.host,
      },
    };

    if (account.network == "ws") {
      proxy.transport = {
        type: "ws",
        path: account.path,
        headers: {
          Host: account.host,
        },
      };
    } else if (account.network == "grpc") {
      proxy.transport = {
        type: "grpc",
        service_name: account.serviceName,
      };
    }
    return proxy;
  }
}

export { Vless };
