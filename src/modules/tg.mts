import { readFileSync } from "fs";
import { Bot as TgBot, InlineKeyboard } from "grammy";
import { FishermanType } from "./fisherman.mjs";
import countryCodeEmoji from "country-code-emoji";

class Bot {
  bot = new TgBot(readFileSync("./bot_token").toString());
  private channelId = "-1001509827144";

  private async make(fisherman: FishermanType, total: number) {
    const account = fisherman.account;

    let message: string = "";
    message += "---------------------------\n";
    message += "Akun Gratis | Free Accounts\n";
    message += "---------------------------\n";
    message += `Jumlah/Count: ${total} 🌸\n`;
    message += `Regional/Region: ${account.region} ${account.cc} ${countryCodeEmoji(account.cc as string)}\n`;
    message += "---------------------------\n";
    message += "Info:\n";
    message += `Type: <code>${account.vpn}</code>\n`;
    message += `Remark: <code>${account.remark}</code>\n`;
    message += `Address: <code>${account.address}</code>\n`;
    message += `Port: <code>${account.port}</code>\n`;
    message += `ID: <code>${account.id}</code>\n`;
    message += `Network: <code>${account.network}</code>\n`;
    message += `Host: <code>${account.host || ""}</code>\n`;
    message += `Path: <code>${account.path || ""}</code>\n`;
    message += `TLS: <code>${account.tls ? true : false}</code>\n`;
    message += `Mode: <code>${account.cdn ? "CDN" : "SNI"}</code>\n`;
    message += `SNI: <code>${account.sni || account.host || ""}</code>\n\n`;
    message += "Origin:\n";
    message += `⌜<code>${fisherman.url}</code>⌟\n\n`;
    message += `Repo: <a href="https://github.com/LatinaHub/LatinaSub">Open repo</a>\n`;
    message += `API: <a href="http://fool.azurewebsites.net">Get subs</a>\n`;
    message += `Join: @v2scrape\n\n`;
    message += `Contact: @d_fordlalatina`;

    return message;
  }

  async sendToChannel(fisherman: FishermanType, total: number) {
    const message = await this.make(fisherman, total);

    await this.bot.api.sendMessage(this.channelId, message, {
      disable_web_page_preview: true,
      parse_mode: "HTML",
      reply_markup: new InlineKeyboard()
        .url("❤️ Donate ❤️", "https://saweria.co/m0qa")
        .row()
        .url("Donators", "https://telegra.ph/Donations-11-05-4")
        .row(),
    });
  }
}

const bot = new Bot();
export { bot };
