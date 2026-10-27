package config

import "testing"

func TestEnvironmentValidation(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "JWT_PUBLIC_KEY_FILE", "JWT_ISSUER", "JWT_AUDIENCE", "JWT_KEY_ID", "IDENTITY_URL"} {
		t.Setenv(key, "configured")
	}
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DB_MAX_CONNS", "")
	t.Setenv("CORS_ORIGINS", "")
	c, err := Load()
	if err != nil || c.MaxConns != 5 || c.Address != ":8080" || c.Origins[0] != "http://localhost:3000" {
		t.Fatal(c, err)
	}
	for _, tc := range []struct{ key, value string }{{"DB_MAX_CONNS", "0"}, {"DB_MAX_CONNS", "51"}, {"DB_MAX_CONNS", "bad"}, {"CORS_ORIGINS", "*"}, {"CORS_ORIGINS", "https://example.com/path"}, {"HTTP_ADDR", "bad"}, {"DATABASE_URL", ""}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("bad configuration accepted")
			}
		})
	}
}
