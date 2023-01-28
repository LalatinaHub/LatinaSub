import { logger, LogLevel } from "./logger.mjs";
import { parse, UrlWithParsedQuery } from "url";
import { decode, encode } from "js-base64";
import { SSRInterface, SSInterface } from "../utils/types.mjs";
import findProcess from "find-process";
import stripAnsi from "strip-ansi";

async function sleep(ms: number) {
  return await new Promise((resolve) => {
    setTimeout(() => {
      resolve(0);
    }, ms);
  });
}

function base64Decode(text: string): string {
  try {
    return decode(text);
  } catch (e: any) {
    logger.log(LogLevel.error, e.name);
  }

  return "";
}

function base64Encode(text: string): string {
  try {
    return encode(text);
  } catch (e: any) {
    logger.log(LogLevel.error, e.name);
  }

  return "";
}

function urlParser(url: string): UrlWithParsedQuery {
  return parse(url, true);
}

async function isRunning(processName: string): Promise<number> {
  const list = await findProcess("name", processName);
  return list.length;
}

function ssrParser(ssr: string): SSRInterface {
  ssr = `ssr://${base64Decode(ssr.replace("ssr://", ""))}`;
  const data = ssr.match(/.+:\d+(.+)\//)?.[1];

  if (data) {
    ssr = ssr.replace(data, "");
    const param = data.split(":");
    ssr += `&protocol=${param[1]}&method=${param[2]}&obfs=${param[3]}&password=${param[4]}`;
  }

  let parsedSsr = urlParser(ssr);
  for (const key of ["password", "group", "remarks", "obfsparam", "param"]) {
    if (parsedSsr.query[key]) {
      parsedSsr.query[key] = base64Decode(parsedSsr.query[key] as string);
    }
  }

  return parsedSsr as SSRInterface;
}

function ssParser(ss: string): SSInterface {
  if (!ss.match("@")) {
    const encodedAuth = ss.match(/:\/\/([\w+\/\+\=]+)/);
    if (encodedAuth && encodedAuth[1]) {
      const authInfo = decode(encodedAuth[1]).split("@");
      const serverInfo = authInfo.pop();
      ss = ss.replace(encodedAuth[1], `${encode(authInfo.join("@"))}@${serverInfo}`);
    }
  }

  ss = decodeURIComponent(ss).replace(/;/g, "&");

  let parsedUrl = parse(ss, true);
  const [method, password] = decode(parsedUrl.auth || "bm90OmZvdW5k").split(":");

  parsedUrl.query = {
    ...parsedUrl.query,
    method,
    password,
  };

  return parsedUrl as SSInterface;
}

function unchalk(text: string | string[]): string {
  let result: string[] = [];
  if (Array.isArray(text)) {
    text.forEach((t) => {
      result.push(stripAnsi(t));
    });

    return result.join("\n");
  } else {
    return stripAnsi(text);
  }
}

export { sleep, base64Decode, base64Encode, urlParser, isRunning, ssrParser, ssParser };
