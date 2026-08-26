package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	Address         string
	DataPath        string
	SelfTest        bool
	SelfTestTimeout time.Duration
}

func parseConfig(args []string) (config, error) {
	defaults, err := addressFromEnvironment()
	if err != nil {
		return config{}, err
	}
	set := flag.NewFlagSet("timber-pest-remediation-ledger", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	var cfg config
	set.StringVar(&cfg.Address, "addr", defaults, "监听地址，必须为 127.0.0.1:<port>")
	set.StringVar(&cfg.DataPath, "data", "./data/ledger.db", "SQLite 数据文件路径")
	set.BoolVar(&cfg.SelfTest, "selftest", false, "运行端到端自检并退出")
	set.DurationVar(&cfg.SelfTestTimeout, "selftest-timeout", 20*time.Second, "自检超时")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("不接受位置参数")
	}
	if err := validateAddress(cfg.Address); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(cfg.DataPath) == "" {
		return config{}, fmt.Errorf("-data 不能为空")
	}
	if cfg.SelfTestTimeout <= 0 || cfg.SelfTestTimeout > 5*time.Minute {
		return config{}, fmt.Errorf("-selftest-timeout 必须大于 0 且不超过 5m")
	}
	return cfg, nil
}

func addressFromEnvironment() (string, error) {
	portText := strings.TrimSpace(os.Getenv("PORT"))
	if portText == "" {
		return defaultAddress, nil
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 必须是明确的 host:port: %w", err)
	}
	if host != "127.0.0.1" {
		return fmt.Errorf("-addr 仅允许绑定 127.0.0.1")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("-addr 端口必须处于 1 到 65535")
	}
	return nil
}
