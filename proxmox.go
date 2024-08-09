package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// https://pve.proxmox.com/wiki/Proxmox_VE_API

const (
	DefaultRemoteViewer = "remote-viewer"
	DefaultPort         = 8006

	APIPath   = "/api2/json/"
	GuestType = "qemu"
)

type ClientConfig struct {
	Host string
	Port int

	remoteViewer string
	kiosk        bool
	fullscreen   bool

	SkipTLSVerify bool

	tokenName  string
	tokenValue string

	autostartVM bool

	debugSpiceSession bool
}

func defaultClientConfig() ClientConfig {
	return ClientConfig{
		Port:          DefaultPort,
		remoteViewer:  DefaultRemoteViewer,
		kiosk:         false,
		fullscreen:    false,
		SkipTLSVerify: false,
		autostartVM:   true,
	}
}

type ProxmoxClient struct {
	ClientConfig

	apiUri string

	client  *http.Client
	headers map[string]string
}

func NewProxmoxClient(host string, c ClientConfig) *ProxmoxClient {
	client := &ProxmoxClient{
		ClientConfig: c,

		apiUri: fmt.Sprintf("https://%s:%d%s", host, c.Port, APIPath),
		client: http.DefaultClient,

		headers: make(map[string]string),
	}

	if client.tokenName != "" {
		client.headers["Authorization"] = fmt.Sprintf("PVEAPIToken=%s=%s", client.tokenName, client.tokenValue)
	}

	// _ = client.get(nil, "access", "permissions")

	return client
}

func (c *ProxmoxClient) get(data any, endpoint ...string) error {
	return c.request("GET", endpoint, data)
}
func (c *ProxmoxClient) post(data any, endpoint ...string) error {
	return c.request("POST", endpoint, data)
}

func (c *ProxmoxClient) request(method string, endpoint []string, data any) error {
	uri, err := url.JoinPath(c.apiUri, endpoint...)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(method, uri, nil)
	if err != nil {
		return err
	}

	for k, v := range c.headers {
		req.Header.Add(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}

	defer resp.Body.Close()

	if data != nil {
		type RespData struct {
			Data any `json:"data"`
		}

		respData := &RespData{Data: data}
		return json.NewDecoder(resp.Body).Decode(&respData)
	} else {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	}
}

func (c *ProxmoxClient) Resources() ([]Resource, error) {
	var resources []Resource
	err := c.get(&resources, "cluster/resources")
	if err != nil {
		return nil, err
	}
	return resources, nil
}

type Resource struct {
	Id     string `json:"id"`
	VmId   int    `json:"vmid"`
	Type   string `json:"type"`
	Node   string `json:"node"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Uptime int    `json:"uptime"`
}

func (c *ProxmoxClient) Status(vm *Resource) (string, error) {
	var stat struct {
		Status string `json:"status"`
	}

	err := c.get(&stat, "nodes", vm.Node, "qemu", strconv.Itoa(vm.VmId), "status", "current")
	if err != nil {
		return "", err
	}
	return stat.Status, nil
}

func (c *ProxmoxClient) Operate(vm *Resource, operation string) error {
	log.Printf("proxmox operate %s %s: %s", vm.Node, vm.Id, operation)
	var jobId string
	err := c.post(&jobId, "nodes", vm.Node, "qemu", strconv.Itoa(vm.VmId), "status", operation)
	if err != nil {
		return err
	}

	var wantStatus string
	switch operation {
	case "start":
		wantStatus = "running"
	case "stop":
		wantStatus = "stopped"
	default:
		return nil
	}

	for {
		time.Sleep(100 * time.Millisecond)
		status, err := c.Status(vm)
		if err != nil {
			return err
		}
		log.Println(status)
		if status == wantStatus {
			return nil
		}
	}
}

func (c *ProxmoxClient) Start(vm *Resource) error { return c.Operate(vm, "start") }
func (c *ProxmoxClient) Stop(vm *Resource) error  { return c.Operate(vm, "stop") }
func (c *ProxmoxClient) Reset(vm *Resource) error { return c.Operate(vm, "reset") }

func (c *ProxmoxClient) SpiceProxy(vm *Resource) error {
	log.Printf("proxmox spiceproxy %s %s", vm.Node, vm.Id)

	if c.autostartVM {
		status, err := c.Status(vm)
		if err != nil {
			return err
		}
		if status != "running" {
			log.Printf("proxmox spiceproxy autostart %s %s", vm.Node, vm.Id)
			err = c.Start(vm)
			if err != nil {
				return err
			}
		}
	}

	keys := make(map[string]any)
	err := c.post(&keys, "nodes", vm.Node, "qemu", strconv.Itoa(vm.VmId), "spiceproxy")
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("[virt-viewer]\n")

	for k, v := range keys {
		sb.WriteString(fmt.Sprintf("%s=%v\n", k, v))
	}

	data := sb.String()

	if c.debugSpiceSession {
		log.Print(data)
	}

	var args []string
	if c.kiosk {
		args = append(args, "--kiosk", "--kiosk-quit", "on-disconnect")
	}
	if c.fullscreen {
		args = append(args, "--full-screen")
	}

	args = append(args, "-")

	cmd := exec.Command(c.remoteViewer, args...)
	cmd.Stdin = strings.NewReader(data)

	return cmd.Run()
}
