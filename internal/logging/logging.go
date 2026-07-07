package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	ConfigMaxSize    = "log.maxsize"
	ConfigMaxBackups = "log.maxbackups"
	ConfigMaxAge     = "log.maxage"
	ConfigLogLevel   = "log.level"
)

var (
	baseDir string

	sinksOnce       sync.Once
	appFileSink     zapcore.WriteSyncer
	internalErrSink zapcore.WriteSyncer
)

func init() {
	// Check for log-config.yaml up front. We need to configure the logging on initialization, so we can't wait for
	// the config presented by the command line flags.
	viper.SetConfigName("log-config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("viper attempted to read log-config.yaml, but failed to read config file: %v\n", err)
		fmt.Println("this may be expected if no config file exists")
	} else {
		fmt.Println("successfully read log-config.yaml config file")
	}

	baseDir = os.Getenv("LOG_DIR")
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("env var LOG_DIR does not exist and home directory could not be determined, using ./logs")
			baseDir = "./logs"
		} else {
			fmt.Println("env var LOG_DIR does not exist, using ~/logs")
			baseDir = filepath.Join(home, "logs")
		}
	}
	err := os.MkdirAll(baseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("error creating logging directory [%s]: %v", baseDir, err)
	}

	viper.SetDefault(ConfigMaxSize, 50)
	viper.SetDefault(ConfigMaxBackups, 5)
	viper.SetDefault(ConfigMaxAge, 30)
	viper.SetDefault(ConfigLogLevel, "debug")
}

func initSinks() {
	// Multiple loggers are created, one for each package, so we want to make sure we have only one sink to manage rollovers, etc.
	sinksOnce.Do(func() {
		appFileSink = zapcore.AddSync(&lumberjack.Logger{
			Filename:   filepath.Join(baseDir, fmt.Sprintf("%s.log", getAppName())),
			MaxSize:    viper.GetInt(ConfigMaxSize),
			MaxBackups: viper.GetInt(ConfigMaxBackups),
			MaxAge:     viper.GetInt(ConfigMaxAge),
		})

		internalErrSink = zapcore.AddSync(&lumberjack.Logger{
			Filename: filepath.Join(baseDir, "zap_internal_errors.log"),
		})
	})
}

// getEnv gets the logging environment from env variable APP_ENV, default 'dev'
func getEnv() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	return env
}

// getAppName gets the app name from env variable APP_NAME, default 'state-machine'
func getAppName() string {
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		appName = "state-machine"
	}
	return appName
}

func getLogLevel(level string) zap.AtomicLevel {
	lvl := zapcore.DebugLevel
	// turn level into a zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		fmt.Printf("Error parsing log level %s, using Debug as default: %v\n", level, err)
	}
	return zap.NewAtomicLevelAt(lvl)
}

// NewLogger creates a new zap.Logger with the given name
// Since it is a common function for the application, logs end up in the same file, simply tagged with a different name
func NewLogger(name string) *zap.Logger {
	initSinks()

	// production encoder config for prod environment to make machine-friendly output
	var config zapcore.EncoderConfig
	if getEnv() == "prod" {
		config = zap.NewProductionEncoderConfig()
	} else {
		config = zap.NewDevelopmentEncoderConfig()
	}

	logLevel := getLogLevel(viper.GetString(ConfigLogLevel))

	errorOutput := zapcore.NewMultiWriteSyncer(
		internalErrSink,
		zapcore.AddSync(os.Stderr),
	)

	var core zapcore.Core
	if getEnv() == "prod" {
		core = zapcore.NewCore(zapcore.NewJSONEncoder(config), appFileSink, logLevel)
	} else {
		devSink := zapcore.NewMultiWriteSyncer(appFileSink, zapcore.AddSync(os.Stdout))
		core = zapcore.NewCore(zapcore.NewConsoleEncoder(config), devSink, logLevel)
	}

	return zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.ErrorOutput(errorOutput),
	).Named(name)
}
