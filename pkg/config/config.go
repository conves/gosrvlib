// Package config handles the configuration of the program.
// The configuration contains the set of initial parameter settings that are read at run-time by the program.
// This package allows to load the configuration from a local file, an environment variable or a remote config provider (e.g. Consul, ETCD, Firestore).
//
// Configuration Loading Strategy:
//
// Different entry points can be used during the development, debugging, testing or deployment.
//
// To get the maximum flexibility, the different configuration entry points can be coordinated in the following sequence (1 has the lowest priority and 5 the maximum):
//
// 1. In the “myprog” program the configuration parameters are defined as a data structure that can be easily mapped to and from a JSON (or YAML) object, and they are initialized with constant default values;
//
//  2. The program attempts to load the local “config.json” configuration file (or what is specified by defaultConfigName and defaultConfigType) and, as soon one is found, overwrites the values previously set. The configuration file is searched in the following ordered directories:
//     ./
//     ~/.myprog/
//     /etc/myprog/
//
//  3. The program attempts to load the environmental variables that define the remote configuration system and, if found, overwrites the correspondent configuration parameters:
//     MYPROG_REMOTECONFIGPROVIDER → remoteConfigProvider
//     MYPROG_REMOTECONFIGENDPOINT → remoteConfigEndpoint
//     MYPROG_REMOTECONFIGPATH → remoteConfigPath
//     MYPROG_REMOTECONFIGSECRETKEYRING → remoteConfigSecretKeyring
//     MYPROG_REMOTECONFIGDATA → remoteConfigData
//
// 4. If the remoteConfigProvider parameter is not empty, the program attempts to load the configuration data from the specified source. This can be any remote source supported by the Viper library (e.g. Consul, ETCD) or alternatively from the MYPROG_REMOTECONFIGDATA environment variable as base64 encoded JSON if MYPROG_REMOTECONFIGPROVIDER is set to "envar".
//
// 5. Any specified command line property overwrites the correspondent configuration parameter.
//
// 6. The configuration parameters are validated via the Validate() function.
//
// An example can be found in examples/service/internal/cli/config.go
package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote" //nolint:revive,nolintlint
)

const (
	defaultConfigName                = "config" // Base name of the file containing the configuration data.
	defaultConfigType                = "json"   // Type of configuration data.
	defaultLogFormat                 = "JSON"
	defaultLogLevel                  = "DEBUG"
	defaultLogAddress                = ""
	defaultLogNetwork                = ""
	defaultRemoteConfigProvider      = ""
	defaultRemoteConfigEndpoint      = ""
	defaultRemoteConfigPath          = ""
	defaultRemoteConfigSecretKeyring = ""

	keyRemoteConfigProvider      = "remoteConfigProvider"
	keyRemoteConfigEndpoint      = "remoteConfigEndpoint"
	keyRemoteConfigPath          = "remoteConfigPath"
	keyRemoteConfigSecretKeyring = "remoteConfigSecretKeyring" //nolint:gosec
	keyRemoteConfigData          = "remoteConfigData"
	keyLogAddress                = "log.address"
	keyLogFormat                 = "log.format"
	keyLogLevel                  = "log.level"
	keyLogNetwork                = "log.network"

	providerEnvVar = "envvar"
)

// Configuration is the interface we need the application config struct to implement.
type Configuration interface {
	SetDefaults(v Viper)
	Validate() error
}

// Viper is the local interface to the actual viper to allow for mocking.
type Viper interface {
	AddConfigPath(in string)
	AddRemoteProvider(provider, endpoint, path string) error
	AddSecureRemoteProvider(provider, endpoint, path, secretkeyring string) error
	AllKeys() []string
	AutomaticEnv()
	BindEnv(input ...string) error
	BindPFlag(key string, flag *pflag.Flag) error
	Get(key string) interface{}
	ReadConfig(in io.Reader) error
	ReadInConfig() error
	ReadRemoteConfig() error
	SetConfigName(in string)
	SetConfigType(in string)
	SetDefault(key string, value interface{})
	SetEnvPrefix(in string)
	Unmarshal(rawVal interface{}, opts ...viper.DecoderConfigOption) error
}

// BaseConfig contains the default configuration options to be used in the application config struct.
type BaseConfig struct {
	// Log configuration.
	Log LogConfig `mapstructure:"log" validate:"required"`
}

// LogConfig contains the configuration for the application logger.
type LogConfig struct {
	// Level is the standard syslog level: EMERGENCY, ALERT, CRITICAL, ERROR, WARNING, NOTICE, INFO, DEBUG.
	Level string `mapstructure:"level" validate:"required,oneof=EMERGENCY ALERT CRITICAL ERROR WARNING NOTICE INFO DEBUG"`

	// Format is the log output format: CONSOLE, JSON.
	Format string `mapstructure:"format" validate:"required,oneof=CONSOLE JSON"`

	// Network is the optional network protocol used to send logs via syslog: udp, tcp.
	Network string `mapstructure:"network" validate:"omitempty,oneof=udp tcp"`

	// Address is the optional remote syslog network address: (ip:port) or just (:port).
	Address string `mapstructure:"address" validate:"omitempty,hostname_port"`
}

// remoteSourceConfig contains the default remote source options to be used in the application config struct.
type remoteSourceConfig struct {
	// Provider is the optional external configuration source: consul, etcd, firestore, envvar.
	// When envvar is set the data shoul dbe set in the Data field.
	Provider string `mapstructure:"remoteConfigProvider" validate:"omitempty,oneof=consul etcd firestore envvar"`

	// Endpoint is the remote configuration URL (ip:port).
	Endpoint string `mapstructure:"remoteConfigEndpoint" validate:"omitempty,url|hostname_port"`

	// Path is the remote configuration path where to search fo the configuration file ("/cli/program").
	Path string `mapstructure:"remoteConfigPath" validate:"omitempty,file"`

	// SecretKeyring is the path to the openpgp secret keyring used to decript the remote configuration data (e.g.: "/etc/program/configkey.gpg")
	SecretKeyring string `mapstructure:"remoteConfigSecretKeyring" validate:"omitempty,file"`

	// Data is the base64 encoded JSON configuration data to be used with the "envvar" provider.
	Data string `mapstructure:"remoteConfigData" validate:"required_if=Provider envar,omitempty,base64"`
}

// Load populates the configuration parameters.
func Load(cmdName, configDir, envPrefix string, cfg Configuration) error {
	localViper := viper.New()
	remoteViper := viper.New()

	return loadConfig(localViper, remoteViper, cmdName, configDir, envPrefix, cfg)
}

// loadConfig loads the configuration.
func loadConfig(localViper, remoteViper Viper, cmdName, configDir, envPrefix string, cfg Configuration) error {
	remoteSourceCfg, err := loadLocalConfig(localViper, cmdName, configDir, envPrefix, cfg)
	if err != nil {
		return fmt.Errorf("failed loading local configuration: %w", err)
	}

	if err := loadRemoteConfig(localViper, remoteViper, remoteSourceCfg, envPrefix, cfg); err != nil {
		return fmt.Errorf("failed loading remote configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("failed validating configuration: %w", err)
	}

	return nil
}

// loadLocalConfig returns the local configuration parameters.
func loadLocalConfig(v Viper, cmdName, configDir, envPrefix string, cfg Configuration) (*remoteSourceConfig, error) {
	// set default remote configuration values
	v.SetDefault(keyRemoteConfigProvider, defaultRemoteConfigProvider)
	v.SetDefault(keyRemoteConfigEndpoint, defaultRemoteConfigEndpoint)
	v.SetDefault(keyRemoteConfigPath, defaultRemoteConfigPath)
	v.SetDefault(keyRemoteConfigSecretKeyring, defaultRemoteConfigSecretKeyring)

	// set default logging configuration values
	v.SetDefault(keyLogFormat, defaultLogFormat)
	v.SetDefault(keyLogLevel, defaultLogLevel)
	v.SetDefault(keyLogAddress, defaultLogAddress)
	v.SetDefault(keyLogNetwork, defaultLogNetwork)

	// set default config name and type
	v.SetConfigName(defaultConfigName)
	v.SetConfigType(defaultConfigType)

	// add default search paths
	configureSearchPath(v, cmdName, configDir)

	// add defaults from application configuration
	cfg.SetDefaults(v)

	// support environment variables for the remote configuration
	v.AutomaticEnv()
	v.SetEnvPrefix(strings.ReplaceAll(envPrefix, "-", "_")) // will be uppercased automatically

	envVar := []string{
		keyRemoteConfigProvider,
		keyRemoteConfigEndpoint,
		keyRemoteConfigPath,
		keyRemoteConfigSecretKeyring,
		keyRemoteConfigData,
	}

	for _, ev := range envVar {
		_ = v.BindEnv(ev) // we ignore the error because we are always passing an argument value
	}

	// Find and read the local configuration file (if any)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed reading in config: %w", err)
	}

	var rsCfg remoteSourceConfig

	if err := v.Unmarshal(&rsCfg); err != nil {
		return nil, fmt.Errorf("failed unmarshalling config: %w", err)
	}

	return &rsCfg, nil
}

// loadRemoteConfig returns the remote configuration parameters.
func loadRemoteConfig(lv Viper, rv Viper, rs *remoteSourceConfig, envPrefix string, cfg Configuration) error {
	for _, k := range lv.AllKeys() {
		rv.SetDefault(k, lv.Get(k))
	}

	rv.SetConfigType(defaultConfigType)

	var err error

	switch rs.Provider {
	case "":
		// ignore remote source
	case providerEnvVar:
		err = loadFromEnvVarSource(rv, rs, envPrefix)
	default:
		err = loadFromRemoteSource(rv, rs, envPrefix)
	}

	if err != nil {
		return fmt.Errorf("failed loading configuration from remote source: %w", err)
	}

	if err := rv.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed loading application configuration: %w", err)
	}

	return nil
}

func loadFromEnvVarSource(v Viper, rc *remoteSourceConfig, envPrefix string) error {
	if rc.Data == "" {
		return validationError(rc.Provider, envPrefix, keyRemoteConfigData)
	}

	data, err := base64.StdEncoding.DecodeString(rc.Data)
	if err != nil {
		return fmt.Errorf("failed decoding config data: %w", err)
	}

	return v.ReadConfig(bytes.NewReader(data)) //nolint:wrapcheck
}

func loadFromRemoteSource(v Viper, rc *remoteSourceConfig, envPrefix string) error {
	if rc.Endpoint == "" {
		return validationError(rc.Provider, envPrefix, keyRemoteConfigEndpoint)
	}

	if rc.Path == "" {
		return validationError(rc.Provider, envPrefix, keyRemoteConfigPath)
	}

	var err error

	if rc.SecretKeyring == "" {
		err = v.AddRemoteProvider(rc.Provider, rc.Endpoint, rc.Path)
	} else {
		err = v.AddSecureRemoteProvider(rc.Provider, rc.Endpoint, rc.Path, rc.SecretKeyring)
	}

	if err != nil {
		return fmt.Errorf("failed adding remote config provider: %w", err)
	}

	return v.ReadRemoteConfig() //nolint:wrapcheck
}

func configureSearchPath(v Viper, cmdName, configDir string) {
	var configSearchPath []string

	if configDir != "" {
		// add the configuration directory specified as program argument
		configSearchPath = append(configSearchPath, configDir)
	}

	// add default search directories for the configuration file
	configSearchPath = append(configSearchPath, []string{
		"./",
		"$HOME/." + cmdName + "/",
		"/etc/" + cmdName + "/",
	}...)

	for _, p := range configSearchPath {
		v.AddConfigPath(p)
	}
}

func validationError(provider, envPrefix, varName string) error {
	return fmt.Errorf("%s config provider requires %s_%s to be set", provider, strings.ToUpper(envPrefix), strings.ToUpper(varName))
}
