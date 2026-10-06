package main

import (
	"adm/internal/app"
	"flag"
	"fmt"
	"os"
)

func main() {
	var listActive bool
	var listAll bool
	flag.BoolVar(&listActive, "l", false, "查看活跃设备")
	flag.BoolVar(&listActive, "list", false, "查看活跃设备")
	flag.BoolVar(&listAll, "al", false, "查看全部设备，包含未启动模拟器")
	flag.BoolVar(&listAll, "all-list", false, "查看全部设备，包含未启动模拟器")
	flag.Parse()

	cli, err := app.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化失败：", err)
		os.Exit(1)
	}

	switch {
	case listAll:
		if err := cli.PrintAllDevices(); err != nil {
			fmt.Fprintln(os.Stderr, "查询失败：", err)
			os.Exit(1)
		}
	case listActive:
		if err := cli.PrintActiveDevices(); err != nil {
			fmt.Fprintln(os.Stderr, "查询失败：", err)
			os.Exit(1)
		}
	default:
		if err := cli.RunInteractive(); err != nil {
			fmt.Fprintln(os.Stderr, "运行失败：", err)
			os.Exit(1)
		}
	}
}
