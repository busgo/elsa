package common

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const (
	DefaultGroup    = "elsa"
	DefaultProtocol = "elsa"
	DefaultHost     = "127.0.0.1:8080"
	DefaultCategory = "providers"
)

// URL
//  elsa://127.0.0.1:8080/io.busgo.trade.TradeService?category=providers&timestamp=1668391366&version=1.0
type URL struct {
	Protocol   string
	Host       string
	Path       string
	Parameters map[string]string
}

func New(protocol, host, path string, parameters map[string]string) URL {

	path = strings.TrimPrefix(path, "/")
	if protocol == "" {
		protocol = DefaultProtocol
	}

	return URL{
		Protocol:   protocol,
		Host:       host,
		Path:       path,
		Parameters: parameters,
	}

}

func (u URL) Parameter(key, defaultValue string) string {

	v := u.Parameters[key]
	if v != "" {
		return v
	}
	return defaultValue
}

func (u URL) String() string {

	keys := make([]string, 0)

	for key, _ := range u.Parameters {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	buf := ""

	for _, key := range keys {
		buf += fmt.Sprintf("&%s=%s", key, u.Parameters[key])
	}
	buf = strings.TrimPrefix(buf, "&")
	return fmt.Sprintf("%s://%s/%s?%s", u.Protocol, u.Host, u.Path, buf)
}

// /elsa/io.busgo.trade.TradeService/providers/elsa%3A%2F%2F127.0.0.1%3A8080%2Fio.busgo.trade.TradeService%3Fcategory%3Dproviders%26timestamp%3D1668391366%26version%3D1.0

func ToRootPath(u URL) string {
	return u.Parameter("group", DefaultGroup)
}

func ToServicePath(u URL) string {
	return fmt.Sprintf("%s/%s", ToRootPath(u), u.Path)
}
func ToCategoryPath(u URL) string {
	return fmt.Sprintf("%s/%s", ToServicePath(u), u.Parameter("category", DefaultCategory))
}

func (u URL) ToUrlPath() string {
	return fmt.Sprintf("/%s/%s", ToCategoryPath(u), u.String())
}

func Decode(raw string) (URL, error) {

	raw = strings.TrimPrefix(raw, "/")
	raw, err := url.QueryUnescape(raw)
	if err != nil {
		return URL{}, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return URL{}, err
	}

	parameters := make(map[string]string)
	for key, values := range u.Query() {
		parameters[key] = values[0]
	}
	return New(u.Scheme, u.Host, u.Path, parameters), nil
}

func ParseURL(raw string, parameters map[string]string) (URL, error) {
	if strings.Contains(raw, "://") {
		raw = DefaultProtocol + "://" + raw
	}

	buf := ""

	for k, v := range parameters {
		buf += fmt.Sprintf("&%s=%s", k, v)
	}

	if len(buf) > 0 {
		buf = buf[1:]
	}

	if strings.Contains(raw, "?") {
		raw += "&" + buf
	} else {
		raw += "?" + buf
	}
	return Decode(raw)
}
