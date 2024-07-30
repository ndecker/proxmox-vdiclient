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

	startStoppedVMs bool

	// TODO: verbose
}

func run(c *Config, args []string) error {
	client := NewProxmoxClient(c.Host, c.ClientConfig)

	var vmName string
	var operation string

	switch len(args) {
	case 0:
		return runGui(c)
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
		stat, err := client.Status(vm.Node, vm.VmId)
		if err != nil {
			return err
		}
		fmt.Println(stat)
	case "open":
		stat, err := client.Status(vm.Node, vm.VmId)
		if err != nil {
			return err
		}

		if stat != "running" {
			log.Printf("starting VM %d %s", vm.VmId, vm.Name)
			err = client.Operate(vm.Node, vm.VmId, "start")
			if err != nil {
				return err
			}

			for {
				time.Sleep(200 * time.Millisecond)

				stat, err = client.Status(vm.Node, vm.VmId)
				if err != nil {
					return err
				}

				if stat == "running" {
					break
				}
				log.Printf("waiting for VM %d %s to start", vm.VmId, vm.Name)
			}
		}

		return client.SpiceProxy(vm.Node, vm.VmId)
	case "start", "stop", "reset":
		return client.Operate(vm.Node, vm.VmId, operation)
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

	flagSet.BoolVar(&config.startStoppedVMs, "start-stopped", true, "start stopped VMs before opening")

	flagSet.BoolVar(&config.debugSpiceSession, "debug-spice-session", false, "debug spice session")
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
