import { UrlWithParsedQuery } from "url";

export type Region = "Asia" | "Europe" | "Africa" | "Oceania" | "Americas";

export interface V2Object {
  vpn: string;
  address: string;
  port: number;
  alterId: number;
  host: string;
  id: string;
  network: string;
  path: string;
  serviceName: string;
  mode: string;
  tls: string;
  type: string;
  security: string;
  skipCertVerify: boolean;
  sni: string;
  plugin?: string;
  pluginParam?: string;
  group?: string;
  obfs?: string;
  obfsParam?: string;
  proto?: string;
  protoParam?: string;
  flow?: string;
  level?: number;
  method?: string;
  ota?: boolean;
  remark: string;
  cdn?: boolean;
  cc?: string;
  error?: string;
  countryName?: string;
  region?: string;
  server?: string;
}

export interface VmessInterface {
  scy: string;
  add: string;
  aid: number;
  host: string;
  id: string;
  net: string;
  path: string;
  port: number;
  ps: string;
  tls: string;
  type: string;
  security: string;
  "skip-cert-verify": boolean;
  sni: string;
  cdn: boolean;
}

export interface TrojanInterface {
  address: string;
  port: number;
  id: string;
  security: string;
  host: string;
  type: string;
  sni: string;
  remark: string;
  path: string;
  cdn: boolean;
  allowInsecure: boolean;
  mode: string;
  serviceName: string;
}

export interface VlessInterface extends TrojanInterface {}

export interface SSInterface extends UrlWithParsedQuery {
  query: {
    plugin: string;
    path: string;
    host: string;
    obfs: string;
    "obfs-host": string;
    tls: string;
    method: string;
    password: string;
  };
}

export interface SSRInterface extends UrlWithParsedQuery {
  query: {
    group: string;
    remarks: string;
    protocol: string;
    protoparam: string;
    method: string;
    obfs: string;
    obfsparam: string;
    password: string;
  };
}

export interface Country {
  name: string;
  code: string;
  region: Region;
}

export interface ConnectServer {
  error?: boolean;
  cc: string;
  cn: string;
  region: string;
  ip?: string;
  server: string;
  message: string;
}

export interface Sub {
  id: number; // Filename
  remarks: string; // Use as credit to thw owner
  site: string; // Use as credit to the owner
  url: string;
  update_method: string; // only change date necessary
  enabled: boolean;
}
