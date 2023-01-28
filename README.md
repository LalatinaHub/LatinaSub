## About

---

<table>
<tr>
<td>

LatinaSub scrape and test v2ray nodes from the internet, keep only working nodes just for you.  
Using sing-box as test tool, give you guarantee 80% good quality nodes.

API is provided, check [LatinaApi](https://github.com/LalatinaHub/LatinaApi)

</td>
</tr>
</table>

## How To Run

- Make folder `bin`
- Download [subconverter](https://github.com/tindy2013/subconverter) and [sing-box](https://github.com/SagerNet/sing-box), put them inside `bin` folder
  - Install `sing-box` with shadowsocksr and grpc support for better result
- Make file called `bot_token`, put your telegram bot token inside
- Change `ChannelId` variable on `./src/modules/tg.mts` to your own channel id
- run
  - `npm install`
  - `npx tsc`
  - `node app/index.js`

## Where Is The Result ?

Looks like you're a dev, but let me say this

<b>
Please make sure you know what you're doing<br/>
Import raw nodes from this repo to v2ray client app, let' s say v2rayNg<br/>
Will leads you to crash in some case.
</b>

---

We have 2 result type, single database and file.  
Both is saved [here](https://github.com/LalatinaHub/LatinaSub/tree/main/result)

### Database Schema

<details>
<summary>Open</summary>

Check [this folder](https://github.com/LalatinaHub/LatinaSub/tree/main/src/modules/format) for complete database schema

</details>

## Credit

- [sing-box](https://github.com/SagerNet/sing-box)
- [subconverter](https://github.com/tindy2013/subconverter)

## License

This software released under [MIT License](https://github.com/LalatinaHub/License/blob/main/LICENSE)
