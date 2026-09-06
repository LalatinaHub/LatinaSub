package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LalatinaHub/common/model"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
)

// BuildSingboxConfig constructs a pure-Go in-memory sing-box Options configuration
// tailored for in-process connectivity testing with local mixed inbound and target outbound.
func BuildSingboxConfig(node *model.ProxyNode, listenPort int, mode Mode, cdnHost, sniHost string) (option.Options, error) {
	var opt option.Options
	if node == nil {
		return opt, fmt.Errorf("proxy node cannot be nil")
	}

	if cdnHost == "" {
		cdnHost = "104.18.2.2"
	}
	if sniHost == "" {
		sniHost = "meet.google.com"
	}

	outbound := nodeToOutboundMap(node)
	applyModeMutation(outbound, node, mode, cdnHost, sniHost)

	outboundTag := outbound["tag"].(string)

	configMap := map[string]any{
		"log": map[string]any{
			"disabled": true,
			"level":    "panic",
		},
		"dns": map[string]any{
			"servers": []map[string]any{
				{
					"tag":    "remote-dns",
					"type":   "udp",
					"server": "1.1.1.1",
				},
			},
			"final": "remote-dns",
		},
		"inbounds": []map[string]any{
			{
				"type":        "mixed",
				"tag":         "mixed-in",
				"listen":      "127.0.0.1",
				"listen_port": listenPort,
			},
		},
		"outbounds": []map[string]any{
			outbound,
			{
				"type": "direct",
				"tag":  "direct",
			},
			{
				"type": "block",
				"tag":  "block",
			},
			{
				"type": "dns",
				"tag":  "dns-out",
			},
		},
		"route": map[string]any{
			"rules": []map[string]any{
				{
					"action":   "hijack-dns",
					"protocol": "dns",
				},
				{
					"action":        "route",
					"outbound":      "direct",
					"ip_is_private": true,
				},
			},
			"final":                 outboundTag,
			"auto_detect_interface": true,
		},
	}

	configBytes, err := json.Marshal(configMap)
	if err != nil {
		return opt, fmt.Errorf("failed to marshal sing-box config map: %w", err)
	}

	ctx := context.Background()
	ctx = box.Context(
		ctx,
		include.InboundRegistry(),
		include.OutboundRegistry(),
		include.EndpointRegistry(),
		include.DNSTransportRegistry(),
		include.ServiceRegistry(),
	)

	if err := opt.UnmarshalJSONContext(ctx, configBytes); err != nil {
		return opt, fmt.Errorf("failed to unmarshal sing-box options: %w", err)
	}

	return opt, nil
}

// nodeToOutboundMap translates model.ProxyNode into a raw sing-box outbound map.
func nodeToOutboundMap(node *model.ProxyNode) map[string]any {
	tag := strings.TrimSpace(node.Remark)
	if tag == "" {
		tag = fmt.Sprintf("proxy-%s", strings.ToLower(node.VPN))
	}

	vpnType := strings.ToLower(strings.TrimSpace(node.VPN))
	if vpnType == "direct" {
		return map[string]any{
			"tag":  tag,
			"type": "direct",
		}
	}

	ob := map[string]any{
		"tag":         tag,
		"server":      node.Server,
		"server_port": node.ServerPort,
	}

	switch vpnType {
	case "shadowsocks", "ss":
		ob["type"] = "shadowsocks"
		method := node.Method
		if method == "" {
			method = "aes-256-gcm"
		}
		ob["method"] = method
		ob["password"] = node.Password
		if node.Plugin != "" {
			ob["plugin"] = node.Plugin
			if node.PluginOpts != "" {
				ob["plugin_opts"] = node.PluginOpts
			}
		}

	case "vmess":
		ob["type"] = "vmess"
		ob["uuid"] = node.UUID
		security := node.Security
		if security == "" {
			security = "auto"
		}
		ob["security"] = security
		ob["alter_id"] = node.AlterID

		applyTLS(ob, node)
		applyTransport(ob, node)

	case "vless":
		ob["type"] = "vless"
		ob["uuid"] = node.UUID

		applyTLS(ob, node)
		applyTransport(ob, node)

	case "trojan":
		ob["type"] = "trojan"
		ob["password"] = node.Password

		applyTLS(ob, node)
		applyTransport(ob, node)

	default:
		// Fallback as shadowsocks or direct
		ob["type"] = "direct"
	}

	return ob
}

func applyTLS(ob map[string]any, node *model.ProxyNode) {
	if node.TLS {
		serverName := node.SNI
		if serverName == "" {
			serverName = node.Host
		}
		if serverName == "" {
			serverName = node.Server
		}

		ob["tls"] = map[string]any{
			"enabled":     true,
			"server_name": serverName,
			"insecure":    true,
		}
	}
}

func applyTransport(ob map[string]any, node *model.ProxyNode) {
	transport := strings.ToLower(strings.TrimSpace(node.Transport))
	switch transport {
	case "ws":
		headers := map[string]any{}
		host := node.Host
		if host == "" {
			host = node.Server
		}
		headers["Host"] = host

		path := node.Path
		if path == "" {
			path = "/"
		}

		ob["transport"] = map[string]any{
			"type":    "ws",
			"path":    path,
			"headers": headers,
		}
	case "grpc":
		ob["transport"] = map[string]any{
			"type":         "grpc",
			"service_name": node.ServiceName,
		}
	}
}

// applyModeMutation mutates outbound based on CDN or SNI mode.
func applyModeMutation(ob map[string]any, node *model.ProxyNode, mode Mode, cdnHost, sniHost string) {
	switch mode {
	case ModeCDN:
		// Override server to CDN IP while retaining original Host header
		ob["server"] = cdnHost

	case ModeSNI:
		// Override TLS SNI and transport Host to bug SNI host
		if tlsVal, ok := ob["tls"].(map[string]any); ok {
			tlsVal["enabled"] = true
			tlsVal["insecure"] = true
			tlsVal["server_name"] = sniHost
			ob["tls"] = tlsVal
		} else if node.TLS {
			ob["tls"] = map[string]any{
				"enabled":     true,
				"insecure":    true,
				"server_name": sniHost,
			}
		}

		if transportVal, ok := ob["transport"].(map[string]any); ok {
			transType, _ := transportVal["type"].(string)
			if strings.EqualFold(transType, "ws") {
				if headersVal, ok := transportVal["headers"].(map[string]any); ok {
					headersVal["Host"] = sniHost
					transportVal["headers"] = headersVal
				} else {
					transportVal["headers"] = map[string]any{
						"Host": sniHost,
					}
				}
				ob["transport"] = transportVal
			}
		}
	}
}

