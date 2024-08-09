package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const ProgName = "proxmox-vdiclient"

type Config struct {
	ClientConfig

	refreshInterval time.Duration
	overrideTheme   string

	// TODO: verbose
}

func run(c *Config, args []string) error {
	client := NewProxmoxClient(c.Host, c.ClientConfig)

	if len(args) == 0 {
		return runGui(c, client)
	} else {
		return runCli(client, args)
	}
}

func runCli(client *ProxmoxClient, args []string) error {
	var vmName, operation string

	switch len(args) {
	case 1:
		vmName = args[0]
		operation = "open"
	case 2:
		vmName = args[0]
		operation = args[1]
	default:
		return fmt.Errorf("invalid number of arguments")
	}

	resources, err := client.Resources()
	if err != nil {
		return err
	}

	vms := filter(resources, func(r Resource) bool { return r.Type == GuestType })

	log.Printf("Found %d guest resources", len(vms))
	for _, vm := range vms {
		log.Printf(" - %d %s (%s)\n", vm.VmId, vm.Name, vm.Status)
	}

	var vm *Resource

	vmId, _ := strconv.Atoi(vmName)
	for _, v := range vms {
		if v.VmId == vmId || v.Name == vmName {
			vm = &v
			break
		}
	}

	if vm == nil {
		return fmt.Errorf("could not find VM %q", vmName)
	}

	switch operation {
	case "status":
		stat, err := client.Status(vm)
		if err != nil {
			return err
		}
		fmt.Println(stat)
	case "open":
		return client.SpiceProxy(vm)
	case "start", "stop", "reset":
		return client.Operate(vm, operation)
	default:
		return fmt.Errorf("invalid operation")
	}

	return nil
}

func main() {
	config := defaultConfig()

	flagSet := flag.NewFlagSet(ProgName, flag.ExitOnError)

	flagSet.Usage = func() {
		fmt.Printf("Usage of %s [flags] [vmid/name [operation]]:\n", ProgName)
		fmt.Println()
		fmt.Println("  operations: status, start, stop, reset, open (default)")
		fmt.Println()
		flagSet.PrintDefaults()

	}

	flagConfigFile := flagSet.String("config", defaultConfigFile(ProgName), "Path to config file")

	flagSet.StringVar(&config.Host, "host", config.Host, "proxmox hostname")
	flagSet.IntVar(&config.Port, "port", config.Port, "proxmox port")

	flagSet.StringVar(&config.remoteViewer, "remote-viewer", config.remoteViewer, "remote viewer executable")
	flagSet.BoolVar(&config.kiosk, "kiosk", config.kiosk, "kiosk mode")
	flagSet.BoolVar(&config.fullscreen, "fullscreen", config.fullscreen, "fullscreen mode")

	flagSet.BoolVar(&config.SkipTLSVerify, "unsafe-skip-tls-verify", config.SkipTLSVerify, "skip TLS certificate verification")

	flagSet.StringVar(&config.tokenName, "token-name", "", "token name")
	flagSet.StringVar(&config.tokenValue, "token-value", "", "token value")

	flagSet.BoolVar(&config.autostartVM, "autostart-vm", config.autostartVM, "start stopped VMs before opening")
	flagSet.DurationVar(&config.refreshInterval, "refresh-interval", config.refreshInterval, "refresh interval")
	flagSet.StringVar(&config.overrideTheme, "override-theme", config.overrideTheme, "override theme (dark/light)")

	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if *flagConfigFile != "" {
		err = loadConfigFile(flagSet, *flagConfigFile)
		if err != nil {
			log.Fatal(err)
		}
	}

	if !filepath.IsAbs(config.remoteViewer) {
		config.remoteViewer, err = exec.LookPath(config.remoteViewer)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = run(config, flagSet.Args())
	if err != nil {
		log.Fatal(err)
	}

}
