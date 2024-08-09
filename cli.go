package main

import (
	"fmt"
	"log"
	"strconv"
)

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
