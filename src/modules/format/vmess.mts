import { db } from "../db.mjs";
import { logger, LogLevel } from "../logger.mjs";
import { V2Object } from "../../utils/types.mjs";
import { base64Decode } from "../helper.mjs";
import { VmessInterface } from "../../utils/types.mjs";

class Vmess {
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
        `CREATE TABLE IF NOT EXISTS Vmess (
                ID INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
                PASSWORD VARCHAR NOT NULL,
                ADDRESS VARCHAR NOT NULL,
                PORT INTEGER NOT NULL ON CONFLICT REPLACE DEFAULT 443,
                SECURITY VARCHAR NOT NULL ON CONFLICT REPLACE DEFAULT 'auto',
                ALTER_ID INT NOT NULL ON CONFLICT REPLACE DEFAULT 0,
                HOST VARCHAR,
                TLS BOOLEAN,
                NETWORK VARCHAR,
                PATH VARCHAR,
                SKIP_CERT_VERIFY BOOLEAN NOT NULL ON CONFLICT REPLACE DEFAULT 1,
                SNI VARCHAR,
                REMARK VARCHAR,
                IS_CDN BOOLEAN,
                CC VARCHAR,
                REGION VARCHAR,
                VPN VARCHAR NOT NULL ON CONFLICT REPLACE DEFAULT 'vmess'
            );`,
        (err: Error) => {
          if (err) {
            logger.log(LogLevel.error, `[${this.account.vpn}] ${err.message}`);
            reject();
          } else {
            logger.log(LogLevel.success, "[DB] Vmess table successfully created!");
            resolve(0);
          }
        }
      );
    }).finally(() => {
      db.close(conn);
    });
  }

  async isExists() {
    const { address, host, network, port, id, path, cdn } = this.account;
    const conn = await db.connect();

    let query = `SELECT * FROM Vmess WHERE PORT=${port} AND PASSWORD='${id}' AND IS_CDN=${
      cdn ? 1 : 0
    } AND NETWORK='${network}'`;
    if (path) query += ` AND PATH='${path}'`;
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

    let query = `INSERT INTO Vmess (
        PASSWORD,
        ADDRESS,
        PORT,
        SECURITY,
        ALTER_ID,
        HOST,
        TLS,
        NETWORK,
        PATH,
        SKIP_CERT_VERIFY,
        SNI,
        REMARK,
        IS_CDN,
        CC,
        REGION
    ) VALUES (
        '${this.account.id}',
        '${this.account.address}',
        ${this.account.port},
        '${this.account.security}',
        ${this.account.alterId},
        '${this.account.host}',
        ${this.account.tls ? 1 : 0},
        '${this.account.network}',
        '${this.account.path}',
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
    const query = "DROP TABLE IF EXISTS Vmess";

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

  // Static method start here
  // Convert to various format
  toV2Object(): any {
    try {
      let config = JSON.parse(base64Decode(this.url.replace("vmess://", ""))) as VmessInterface;

      return {
        vpn: "vmess",
        address: config.add,
        port: parseInt(`${config.port}`),
        alterId: parseInt(`${config.aid}`) || 0,
        host: config.host,
        id: config.id,
        network: config.net,
        path: config.path,
        tls: config.tls,
        type: config.type,
        security: config.security || "auto",
        skipCertVerify: true,
        sni: config.sni || config.host,
        remark: config.ps,
      } as V2Object;
    } catch (e: any) {
      return { vpn: "vmess", error: e.message } as V2Object;
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
      security: "auto",
      alter_id: account.alterId || 0,
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
        service_name: account.path,
      };
    }

    return proxy;
  }
}

export { Vmess };
