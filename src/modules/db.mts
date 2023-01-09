import sqlite3 from "sqlite3";
import { existsSync, mkdirSync } from "fs";
import { sleep } from "./helper.mjs";
import { logger, LogLevel } from "./logger.mjs";

const sql = sqlite3.verbose(); // Delete .verbose() on production

class Database {
  private _sqlitePool: sqlite3.Database[] = [];
  private _maxDatabaseConn: number = 50;
  private _databasePath: string = "./result/db.sqlite";

  constructor() {
    if (!existsSync("./result")) mkdirSync("./result", { recursive: true });
  }

  set databasePath(path: string) {
    this._databasePath = path;
  }

  async make(connection: number = 10) {
    // Close all existing connection
    while (this._sqlitePool.length) {
      this._sqlitePool.shift()?.close();
    }

    for (let i = 1; i <= connection; i++) {
      await new Promise((resolve) => {
        resolve(new sql.Database(this._databasePath));
      }).then((res) => {
        this._sqlitePool.push(res as sqlite3.Database);
      });
    }

    console.log(`🗄 Total database connection successfully created: ${this._sqlitePool.length}`);
  }

  async connect(retry: number = 5): Promise<sqlite3.Database> {
    if (retry) {
      if (this._sqlitePool.length) {
        return this._sqlitePool.shift() as sqlite3.Database;
      } else {
        await sleep(1000);
        return await this.connect(retry - 1).then((res) => {
          if (res) return res;
          else throw new Error("Connection to Database failed!");
        });
      }
    } else {
      // Create new database conection to satisfy demands
      logger.log(LogLevel.info, "New database connection established!");
      return new sql.Database(this._databasePath);
    }
  }

  close(connection: sqlite3.Database | undefined) {
    if (connection) this._sqlitePool.push(connection);

    // Remove over created database connection
    while (this._sqlitePool.length > this._maxDatabaseConn) {
      logger.log(LogLevel.info, "High database connection!");
      this._sqlitePool.pop()?.close();
    }
  }
}

const db = new Database();
export { db };
