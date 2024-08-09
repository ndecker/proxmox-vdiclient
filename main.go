package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

const ProgName = "proxmox-vdiclient"

func main() {
	clientConfig := DefaultClientConfig()
	clientConfig.LogPrintf = log.Printf

	guiConfig := DefaultGuiConfig()
	guiConfig.LogPrintf = log.Printf

	flagSet := flag.NewFlagSet(ProgName, flag.ExitOnError)

	flagSet.Usage = func() {
		fmt.Printf("Usage of %s [flags] [vmid/name [operation]]:\n", ProgName)
		fmt.Println()
		fmt.Println("  operations: status, start, stop, reset, open (default)")
		fmt.Println()
		flagSet.PrintDefaults()

	}

	flagSet.StringVar(&guiConfig.title, "title", guiConfig.title, "title shown in gui")

	flagSet.StringVar(&clientConfig.Host, "host", clientConfig.Host, "proxmox hostname")
	flagSet.IntVar(&clientConfig.Port, "port", clientConfig.Port, "proxmox port")

	flagSet.StringVar(&clientConfig.remoteViewer, "remote-viewer", clientConfig.remoteViewer, "remote viewer executable")
	flagSet.BoolVar(&clientConfig.kiosk, "kiosk", clientConfig.kiosk, "kiosk mode")
	flagSet.BoolVar(&clientConfig.fullscreen, "fullscreen", clientConfig.fullscreen, "fullscreen mode")

	flagSet.BoolVar(&clientConfig.SkipTLSVerify, "unsafe-skip-tls-verify", clientConfig.SkipTLSVerify, "skip TLS certificate verification")

	flagSet.StringVar(&clientConfig.tokenName, "token-name", "", "token name")
	flagSet.StringVar(&clientConfig.tokenValue, "token-value", "", "token value")

	flagSet.BoolVar(&clientConfig.autostartVM, "autostart-vm", clientConfig.autostartVM, "start stopped VMs before opening")

	flagSet.DurationVar(&guiConfig.refreshInterval, "refresh-interval", guiConfig.refreshInterval, "refresh interval")
	flagSet.StringVar(&guiConfig.overrideTheme, "override-theme", guiConfig.overrideTheme, "override theme (dark/light)")

	flagConfigFile := &ConfigFileFlag{FlagSet: flagSet, LogPrintf: log.Printf}
	checkFatal(flagConfigFile.ReadDefault(ProgName))
	flagSet.Var(flagConfigFile, "config", "Path to config file")

	checkFatal(flagSet.Parse(os.Args[1:]))

	client, err := NewProxmoxClient(clientConfig)
	checkFatal(err)

	if len(flagSet.Args()) == 0 {
		runGui(guiConfig, client)
	} else {
		checkFatal(runCli(client, flagSet.Args()))
	}
}

func checkFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
