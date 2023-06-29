package config

// Sample is a config sample
type Sample struct {
	API         APIConfig         `mapstructure:"api"`
	DB          DatabaseConfig    `mapstructure:"db"`
	FeatureGate FeatureGateConfig `mapstructure:"feature_gate"`
}

// API returns the API config
func API() APIConfig {
	return Proxy.Get().(Sample).API
}

type APIConfig struct {
	ServiceXXXAddress string `mapstructure:"service_xxx_address"`
}

// DB returns the database config
func DB() DatabaseConfig {
	return Proxy.Get().(Sample).DB
}

type DatabaseConfig struct {
	Address  string `mapstructure:"address"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// FeatureGate returns the feature gate config
func FeatureGate() FeatureGateConfig {
	return Proxy.Get().(Sample).FeatureGate
}

type FeatureGateConfig struct {
	EnableXXX bool `mapstructure:"enable_xxx"`
}
