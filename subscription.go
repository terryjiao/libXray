package libXray

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"net/url"

	"github.com/xtls/xray-core/common/platform/filesystem"
)

func ConvertXrayJsonToShareText(xrayPath string, textPath string) string {
	xrayBytes, err := filesystem.ReadFile(xrayPath)
	if err != nil {
		return err.Error()
	}

	var xray xrayJson

	err = json.Unmarshal(xrayBytes, &xray)
	if err != nil {
		return err.Error()
	}

	outbounds := xray.flattenOutbounds()
	if len(outbounds) == 0 {
		return "no valid outbounds"
	}

	var links []string
	for _, outbound := range outbounds {
		link, err := outbound.shareLink()
		if err == nil {
			links = append(links, link.String())
		}
	}
	if len(links) == 0 {
		return "no valid outbounds"
	}
	text := strings.Join(links, "\n")
	err = writeText(text, textPath)
	if err != nil {
		return err.Error()
	}

	return ""
}

func (proxy xrayOutbound) shareLink() (*url.URL, error) {
	var shareUrl url.URL

	switch proxy.Protocol {
	case "shadowsocks":
		err := proxy.shadowsocksLink(&shareUrl)
		if err != nil {
			return nil, err
		}
	case "vmess":
		err := proxy.vmessLink(&shareUrl)
		if err != nil {
			return nil, err
		}
	case "vless":
		err := proxy.vlessLink(&shareUrl)
		if err != nil {
			return nil, err
		}
	case "socks":
		err := proxy.socksLink(&shareUrl)
		if err != nil {
			return nil, err
		}
	case "trojan":
		err := proxy.trojanLink(&shareUrl)
		if err != nil {
			return nil, err
		}
	}
	proxy.streamSettingsQuery(&shareUrl)

	return &shareUrl, nil
}

func (proxy xrayOutbound) shadowsocksLink(link *url.URL) error {
	var settings xrayShadowsocks
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = proxy.Name
	link.Scheme = "ss"

	for _, server := range settings.Servers {
		link.Host = fmt.Sprintf("%s:%d", server.Address, server.Port)
		password := fmt.Sprintf("%s:%s", server.Password, server.Method)
		username := base64.StdEncoding.EncodeToString([]byte(password))
		link.User = url.User(username)
	}
	return nil
}

func (proxy xrayOutbound) vmessLink(link *url.URL) error {
	var settings xrayVMess
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = proxy.Name
	link.Scheme = "vmess"

	for _, vnext := range settings.Vnext {
		link.Host = fmt.Sprintf("%s:%d", vnext.Address, vnext.Port)
		for _, user := range vnext.Users {
			link.User = url.User(user.Id)
			link.RawQuery = addQuery(link.RawQuery, "encryption", user.Security)
		}
	}
	return nil
}

func (proxy xrayOutbound) vlessLink(link *url.URL) error {
	var settings xrayVLESS
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = proxy.Name
	link.Scheme = "vless"

	for _, vnext := range settings.Vnext {
		link.Host = fmt.Sprintf("%s:%d", vnext.Address, vnext.Port)
		for _, user := range vnext.Users {
			link.User = url.User(user.Id)
			if len(user.Flow) > 0 {
				link.RawQuery = addQuery(link.RawQuery, "flow", user.Flow)
			}
		}
	}
	return nil
}

func (proxy xrayOutbound) socksLink(link *url.URL) error {
	var settings xraySocks
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = proxy.Name
	link.Scheme = "socks"

	for _, server := range settings.Servers {
		link.Host = fmt.Sprintf("%s:%d", server.Address, server.Port)
		if len(server.Users) == 0 {
			username := base64.StdEncoding.EncodeToString([]byte(":"))
			link.User = url.User(username)
		} else {
			for _, user := range server.Users {
				password := fmt.Sprintf("%s:%s", user.User, user.Pass)
				username := base64.StdEncoding.EncodeToString([]byte(password))
				link.User = url.User(username)
			}
		}
	}
	return nil
}

func (proxy xrayOutbound) trojanLink(link *url.URL) error {
	var settings xrayTrojan
	err := json.Unmarshal(*proxy.Settings, &settings)
	if err != nil {
		return err
	}

	link.Fragment = proxy.Name
	link.Scheme = "trojan"

	for _, server := range settings.Servers {
		link.Host = fmt.Sprintf("%s:%d", server.Address, server.Port)
		link.User = url.User(server.Password)
	}
	return nil
}

func (proxy xrayOutbound) streamSettingsQuery(link *url.URL) {
	streamSettings := proxy.StreamSettings
	if streamSettings == nil {
		return
	}
	query := link.RawQuery

	if len(streamSettings.Network) == 0 {
		streamSettings.Network = "tcp"
	}
	query = addQuery(query, "type", streamSettings.Network)

	if len(streamSettings.Security) == 0 {
		streamSettings.Security = "none"
	}
	query = addQuery(query, "security", streamSettings.Security)

	switch streamSettings.Network {
	case "tcp":
		if streamSettings.TcpSettings == nil || streamSettings.TcpSettings.Header == nil {
			break
		}
		header := streamSettings.TcpSettings.Header
		headerType := header.Type
		if len(headerType) > 0 {
			query = addQuery(query, "headerType", headerType)
			if header.Request == nil {
				break
			}
			path := header.Request.Path
			if len(path) > 0 {
				query = addQuery(query, "path", strings.Join(path, ","))
			}
			if header.Request.Headers == nil {
				break
			}
			host := streamSettings.TcpSettings.Header.Request.Headers.Host
			if len(host) > 0 {
				query = addQuery(query, "host", strings.Join(host, ","))
			}
		}
	case "kcp":
		if streamSettings.KcpSettings == nil {
			break
		}
		seed := streamSettings.KcpSettings.Seed
		if len(seed) > 0 {
			query = addQuery(query, "seed", seed)
		}
		if streamSettings.KcpSettings.Header == nil {
			break
		}
		headerType := streamSettings.KcpSettings.Header.Type
		if len(headerType) > 0 {
			query = addQuery(query, "headerType", headerType)
		}
	case "ws":
		if streamSettings.WsSettings == nil {
			break
		}
		path := streamSettings.WsSettings.Path
		if len(path) > 0 {
			query = addQuery(query, "path", path)
		}
		if streamSettings.WsSettings.Headers == nil {
			break
		}
		host := streamSettings.WsSettings.Headers.Host
		if len(host) > 0 {
			query = addQuery(query, "host", host)
		}
	case "grpc":
		if streamSettings.GrpcSettings == nil {
			break
		}
		mode := streamSettings.GrpcSettings.MultiMode
		if mode {
			query = addQuery(query, "mode", "multi")
		} else {
			query = addQuery(query, "mode", "gun")
		}
		serviceName := streamSettings.GrpcSettings.ServiceName
		if len(serviceName) > 0 {
			query = addQuery(query, "serviceName", serviceName)
		}
	case "quic":
		if streamSettings.QuicSettings == nil {
			break
		}
		quicSecurity := streamSettings.QuicSettings.Security
		if len(quicSecurity) > 0 {
			query = addQuery(query, "quicSecurity", quicSecurity)
		}
		key := streamSettings.QuicSettings.Key
		if len(key) > 0 {
			query = addQuery(query, "key", key)
		}
		if streamSettings.KcpSettings.Header == nil {
			break
		}
		headerType := streamSettings.KcpSettings.Header.Type
		if len(headerType) > 0 {
			query = addQuery(query, "headerType", headerType)
		}

	case "http":
		if streamSettings.HttpSettings == nil {
			break
		}
		host := streamSettings.HttpSettings.Host
		if len(host) > 0 {
			query = addQuery(query, "host", strings.Join(host, ","))
		}
		path := streamSettings.HttpSettings.Path
		if len(path) > 0 {
			query = addQuery(query, "path", path)
		}
	}

	switch streamSettings.Security {
	case "tls":
		if streamSettings.TlsSettings == nil {
			break
		}
		fp := streamSettings.TlsSettings.Fingerprint
		if len(fp) > 0 {
			query = addQuery(query, "fp", fp)
		}
		sni := streamSettings.TlsSettings.ServerName
		if len(sni) > 0 {
			query = addQuery(query, "sni", sni)
		}
		alpn := streamSettings.TlsSettings.Alpn
		if len(alpn) > 0 {
			query = addQuery(query, "alpn", strings.Join(alpn, ","))
		}
	case "reality":
		if streamSettings.RealitySettings == nil {
			break
		}
		fp := streamSettings.RealitySettings.Fingerprint
		if len(fp) > 0 {
			query = addQuery(query, "fp", fp)
		}
		sni := streamSettings.RealitySettings.ServerName
		if len(sni) > 0 {
			query = addQuery(query, "sni", sni)
		}
		pbk := streamSettings.RealitySettings.PublicKey
		if len(pbk) > 0 {
			query = addQuery(query, "pbk", pbk)
		}
		sid := streamSettings.RealitySettings.ShortId
		if len(sid) > 0 {
			query = addQuery(query, "sid", sid)
		}
		spx := streamSettings.RealitySettings.SpiderX
		if len(spx) > 0 {
			query = addQuery(query, "spx", spx)
		}
	}

	link.RawQuery = query
}

func addQuery(query string, key string, value string) string {
	newQuery := fmt.Sprintf("%s=%s", key, url.QueryEscape(value))
	if len(query) == 0 {
		return newQuery
	} else {
		return fmt.Sprintf("%s&%s", query, newQuery)
	}
}
