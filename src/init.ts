import { existsSync, mkdirSync, stat, writeFileSync } from "fs";

// Initialize
if (!existsSync("./result")) mkdirSync("./result");
if (!existsSync("./result/blacklist_node")) writeFileSync("./result/blacklist_node", "");
if (!existsSync("./result/blacklist_sub")) writeFileSync("./result/blacklist_sub", "");
if (process.env.BOT_TOKEN) writeFileSync("./bot_token", process.env.BOT_TOKEN);
if (process.env.CHANNEL_ID) writeFileSync("./channel_id", process.env.CHANNEL_ID);

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
