import { existsSync, mkdirSync, stat, writeFileSync } from "fs";

// Initialize
if (!existsSync("./result")) mkdirSync("./result");
if (!existsSync("./result/blacklist")) writeFileSync("./result/blacklist", "");

stat("./result/blacklist", (e, s) => {
  if (e) {
    console.error(e);
    return;
  }

  // Reset blacklist each day
  const lastDate = new Date(s.mtime);
  const nowDate = new Date();

  if (lastDate.getDate() != nowDate.getDate()) {
    writeFileSync("./result/blacklist", "");
  }
});
