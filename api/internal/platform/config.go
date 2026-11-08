package platform

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv            string
	SiteURL           string
	APIPort           int
	DatabaseURL       string
	JWTSecret         []byte
	AccessTTL         time.Duration
	RefreshTTL        time.Duration
	CookieSecure      bool
	CookieSameSite    string
	RefreshCookiePath string
	AllowedOrigins    []string
	CORSAllowed       []string
	TrustedProxies    []*net.IPNet
	WhatsAppNumber    string
	StoreName         string
	StoreTZ           *time.Location
	AdminEmail        string
	AdminPassword     string
	TOTPEncKey        []byte
	UploadDir         string
	WebInternalURL    string
	RevalidateSecret  string
}

func loadDotEnv(paths ...string) {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				v = strings.Trim(v, `"'`)
				if os.Getenv(k) == "" {
					_ = os.Setenv(k, v)
				}
			}
		}
		break
	}
}

func LoadConfig() (*Config, error) {
	loadDotEnv(".env", "../.env", "api/.env")
	var missing []string
	getReq := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}
	getOpt := func(key, def string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			return def
		}
		return v
	}

	appEnv := getOpt("APP_ENV", "development")
	siteURL := getReq("SITE_URL")
	portStr := getOpt("API_PORT", "8080")
	dbURL := getReq("DATABASE_URL")
	jwtSecret := getReq("JWT_SECRET")
	accessTTLStr := getOpt("ACCESS_TTL", "15m")
	refreshTTLStr := getOpt("REFRESH_TTL", "720h")
	cookieSecureStr := getOpt("COOKIE_SECURE", "false")
	cookieSameSite := getOpt("COOKIE_SAMESITE", "Lax")
	refreshPath := getOpt("REFRESH_COOKIE_PATH", "/")
	allowedOriginsStr := getReq("ALLOWED_ORIGINS")
	corsAllowedStr := getOpt("CORS_ALLOWED_ORIGINS", "")
	trustedProxiesStr := getOpt("TRUSTED_PROXIES", "127.0.0.1/32,::1/128,172.16.0.0/12,10.0.0.0/8,192.168.0.0/16")
	waNumber := getReq("WHATSAPP_NUMBER")
	storeName := getOpt("STORE_NAME", "Global Mandiri Computer")
	storeTZStr := getOpt("STORE_TZ", "Asia/Jakarta")
	totpKeyB64 := getOpt("TOTP_ENC_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	uploadDir := getOpt("UPLOAD_DIR", "./data/uploads")
	webInternalURL := getOpt("WEB_INTERNAL_URL", siteURL)
	revalidateSecret := getOpt("REVALIDATE_SECRET", "gmc-internal-revalidate-secret-2026")

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return nil, fmt.Errorf("invalid API_PORT: %s", portStr)
	}

	accessTTL, err := time.ParseDuration(accessTTLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ACCESS_TTL: %w", err)
	}
	refreshTTL, err := time.ParseDuration(refreshTTLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid REFRESH_TTL: %w", err)
	}

	cookieSecure, err := strconv.ParseBool(cookieSecureStr)
	if err != nil {
		return nil, fmt.Errorf("invalid COOKIE_SECURE: %w", err)
	}

	if appEnv == "production" {
		if !cookieSecure {
			return nil, fmt.Errorf("production guard: COOKIE_SECURE must be true when APP_ENV=production")
		}
		if jwtSecret == "change-me-min-32-chars" || strings.Contains(jwtSecret, "gmc-dev-jwt-secret") {
			return nil, fmt.Errorf("production guard: default JWT_SECRET is forbidden when APP_ENV=production")
		}
	}

	loc, err := time.LoadLocation(storeTZStr)
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	totpKey, err := base64.StdEncoding.DecodeString(totpKeyB64)
	if err != nil || len(totpKey) != 32 {
		totpKey = []byte("0123456789abcdef0123456789abcdef")
	}

	var trustedNets []*net.IPNet
	for _, cidr := range strings.Split(trustedProxiesStr, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if !strings.Contains(cidr, "/") {
			if strings.Contains(cidr, ":") {
				cidr += "/128"
			} else {
				cidr += "/32"
			}
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR in TRUSTED_PROXIES (%s): %w", cidr, err)
		}
		trustedNets = append(trustedNets, ipNet)
	}

	splitCSV := func(s string) []string {
		var out []string
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}

	return &Config{
		AppEnv:            appEnv,
		SiteURL:           strings.TrimRight(siteURL, "/"),
		APIPort:           port,
		DatabaseURL:       dbURL,
		JWTSecret:         []byte(jwtSecret),
		AccessTTL:         accessTTL,
		RefreshTTL:        refreshTTL,
		CookieSecure:      cookieSecure,
		CookieSameSite:    cookieSameSite,
		RefreshCookiePath: refreshPath,
		AllowedOrigins:    splitCSV(allowedOriginsStr),
		CORSAllowed:       splitCSV(corsAllowedStr),
		TrustedProxies:    trustedNets,
		WhatsAppNumber:    NormalizePhoneOrKeep(waNumber),
		StoreName:         storeName,
		StoreTZ:           loc,
		AdminEmail:        strings.TrimSpace(os.Getenv("ADMIN_EMAIL")),
		AdminPassword:     os.Getenv("ADMIN_PASSWORD"),
		TOTPEncKey:        totpKey,
		UploadDir:         uploadDir,
		WebInternalURL:    strings.TrimRight(webInternalURL, "/"),
		RevalidateSecret:  revalidateSecret,
	}, nil
}
