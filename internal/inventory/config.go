package inventory

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var hostAttributeNames map[string]string
var txtKeysSeparator string

// tsigAlgo processes user-supplied TSIG algorithm names.
func tsigAlgo(algo string) string {
	switch algo {
	case "hmac-sha1", "hmac-sha224", "hmac-sha256", "hmac-sha384", "hmac-sha512":
		return algo + "."
	default:
		return "hmac-sha256."
	}
}

// initConfig initializes the configuration.
func initConfig() (Config, error) {
	v := viper.New()

	// Load YAML configuration.
	path, ok := os.LookupEnv("ADI_CONFIG_FILE")
	if ok {
		// Load a specific config file.
		v.SetConfigFile(path)
	} else {
		// Try to find the config file in standard loctions.
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine user's home directory")
		}

		v.SetConfigName("ansible-dns-inventory")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath(home + "/.ansible")
		v.AddConfigPath("/etc/ansible")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, errors.Wrap(err, "failed to read config file")
		}
	}

	// Setup environment variables handling.
	v.SetEnvPrefix("adi")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults.
	v.SetDefault("dns.server", "127.0.0.1:53")
	v.SetDefault("dns.timeout", "30s")
	v.SetDefault("dns.zones", []string{"server.local."})

	v.SetDefault("dns.notransfer.enabled", false)
	v.SetDefault("dns.notransfer.host", "ansible-dns-inventory")
	v.SetDefault("dns.notransfer.separator", ":")

	v.SetDefault("dns.tsig.enabled", false)
	v.SetDefault("dns.tsig.key", "axfr.")
	v.SetDefault("dns.tsig.secret", "c2VjcmV0Cg==")
	v.SetDefault("dns.tsig.algo", "hmac-sha256")

	v.SetDefault("txt.kv.separator", ";")
	v.SetDefault("txt.kv.equalsign", "=")

	v.SetDefault("txt.vars.enabled", false)
	v.SetDefault("txt.vars.separator", ",")
	v.SetDefault("txt.vars.equalsign", "=")

	v.SetDefault("txt.keys.separator", "_")
	v.SetDefault("txt.keys.os", "OS")
	v.SetDefault("txt.keys.env", "ENV")
	v.SetDefault("txt.keys.role", "ROLE")
	v.SetDefault("txt.keys.srv", "SRV")
	v.SetDefault("txt.keys.vars", "VARS")

	// Process user-supplied TSIG algorithm name.
	v.Set("dns.tsig.algo", tsigAlgo(v.GetString("dns.tsig.algo")))

	hostAttributeNames = make(map[string]string)
	hostAttributeNames["OS"] = v.GetString("txt.keys.os")
	hostAttributeNames["ENV"] = v.GetString("txt.keys.env")
	hostAttributeNames["ROLE"] = v.GetString("txt.keys.role")
	hostAttributeNames["SRV"] = v.GetString("txt.keys.srv")
	hostAttributeNames["VARS"] = v.GetString("txt.keys.vars")

	txtKeysSeparator = v.GetString("txt.keys.separator")

	return v, nil
}
