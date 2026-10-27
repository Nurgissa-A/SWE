package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL, Address, KeyFile, Issuer, Audience, KeyID, IdentityURL string
	MaxConns                                                            int32
	Origins                                                             []string
}

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), Address: os.Getenv("HTTP_ADDR"), KeyFile: os.Getenv("JWT_PUBLIC_KEY_FILE"), Issuer: os.Getenv("JWT_ISSUER"), Audience: os.Getenv("JWT_AUDIENCE"), KeyID: os.Getenv("JWT_KEY_ID"), IdentityURL: os.Getenv("IDENTITY_URL"), MaxConns: 5}
	if c.Address == "" {
		c.Address = ":8080"
	}
	if _, _, err := net.SplitHostPort(c.Address); err != nil {
		return c, errors.New("invalid HTTP_ADDR")
	}
	if c.DatabaseURL == "" || c.KeyFile == "" || c.Issuer == "" || c.Audience == "" || c.KeyID == "" || c.IdentityURL == "" {
		return c, errors.New("DATABASE_URL, JWT key/issuer/audience/key ID and IDENTITY_URL are required")
	}
	if raw := os.Getenv("DB_MAX_CONNS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 50 {
			return c, errors.New("DB_MAX_CONNS must be 1..50")
		}
		c.MaxConns = int32(n)
	}
	origins := os.Getenv("CORS_ORIGINS")
	if origins == "" {
		origins = "http://localhost:3000"
	}
	for _, origin := range strings.Split(origins, ",") {
		origin = strings.TrimSpace(origin)
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return c, errors.New("CORS_ORIGINS must contain comma-separated http(s) origins")
		}
		c.Origins = append(c.Origins, origin)
	}
	return c, nil
}
