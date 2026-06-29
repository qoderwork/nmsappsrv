package config

import (
	"fmt"
	"strings"

	"nmsappsrv/pkg/logger"
	"github.com/spf13/viper"
)

type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	DB      DatabaseConfig `mapstructure:"database"`
	Redis   RedisConfig   `mapstructure:"redis"`
	Logger  LoggerConfig  `mapstructure:"logger"`
	TR069   TR069Config   `mapstructure:"tr069"`
	SNMP    SNMPConfig    `mapstructure:"snmp"`
	MQ      MQConfig      `mapstructure:"mq"`
}

type ServerConfig struct {
	Name string `mapstructure:"name"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	Charset      string `mapstructure:"charset"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	LogLevel     string `mapstructure:"log_level"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type LoggerConfig struct {
	Filename      string `mapstructure:"filename"`
	Level         string `mapstructure:"level"`
	MaxSizeMB     int    `mapstructure:"max_size_mb"`
	MaxBackups    int    `mapstructure:"max_backups"`
	RetentionDays int    `mapstructure:"retention_days"`
	Compress      bool   `mapstructure:"compress"`
	Stdout        bool   `mapstructure:"stdout"`
}

type TR069Config struct {
	ACSUrl                    string `mapstructure:"acs_url"`
	InformInterval            int    `mapstructure:"inform_interval"`
	ConnectionTimeout         int    `mapstructure:"connection_timeout"`
	UDPConnectionRequestPort  int    `mapstructure:"udp_connection_request_port"`
}

type SNMPConfig struct {
	TrapListenPort   int    `mapstructure:"trap_listen_port"`
	EnterpriseOID    string `mapstructure:"enterprise_oid"`
	DefaultVersion   string `mapstructure:"default_version"`
	DefaultCommunity string `mapstructure:"default_community"`
}

type MQConfig struct {
	OperationQueue   string `mapstructure:"operation_queue"`
	InformQueue      string `mapstructure:"inform_queue"`
	EventResultQueue string `mapstructure:"event_result_queue"`
	AlarmQueue       string `mapstructure:"alarm_queue"`
	SNMPQueue        string `mapstructure:"snmp_queue"`
	WebCallbackQueue string `mapstructure:"web_callback_queue"`
	PMQueue          string `mapstructure:"pm_queue"`
}

var Cfg *Config

func Load(configPath ...string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if len(configPath) > 0 {
		v.AddConfigPath(configPath[0])
	}
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")

	// 环境变量覆盖
	v.SetEnvPrefix("NMS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	Cfg = &Config{}
	if err := v.Unmarshal(Cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 初始化日志
	logger.Init(logger.Config{
		Filename:      Cfg.Logger.Filename,
		Level:         Cfg.Logger.Level,
		MaxSizeMB:     Cfg.Logger.MaxSizeMB,
		MaxBackups:    Cfg.Logger.MaxBackups,
		RetentionDays: Cfg.Logger.RetentionDays,
		Compress:      Cfg.Logger.Compress,
		Stdout:        Cfg.Logger.Stdout,
	})

	logger.Infof("config loaded: server=%s, port=%d", Cfg.Server.Name, Cfg.Server.Port)
	return Cfg, nil
}
