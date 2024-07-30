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

	debugSpiceSession bool
}

func defaultClientConfig() ClientConfig {
	return ClientConfig{
		Port:          DefaultPort,
		remoteViewer:  DefaultRemoteViewer,
		kiosk:         false,
		fullscreen:    false,
		SkipTLSVerify: false,
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

func (c *ProxmoxClient) Nodes() ([]Node, error) {
	var nodes []Node
	err := c.get(&nodes, "nodes")
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

type Node struct {
	SSLFingerprint string `json:"ssl_fingerprint"`
	Level          string `json:"level"`
	Type           string `json:"type"`
	Node           string `json:"node"`
	Status         string `json:"status"`
	Id             string `json:"id"`
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

func (c *ProxmoxClient) Status(node string, vmid int) (string, error) {
	var stat struct {
		Status string `json:"status"`
	}

	err := c.get(&stat, "nodes", node, "qemu", strconv.Itoa(vmid), "status", "current")
	if err != nil {
		return "", err
	}
	return stat.Status, nil
}

func (c *ProxmoxClient) Operate(node string, vmid int, operation string) error {
	var jobId string
	err := c.post(&jobId, "nodes", node, "qemu", strconv.Itoa(vmid), "status", operation)
	// fmt.Println(jobId)
	return err
}

func (c *ProxmoxClient) SpiceProxy(node string, vmid int) error {
	keys := make(map[string]any)
	err := c.post(&keys, "nodes", node, "qemu", strconv.Itoa(vmid), "spiceproxy")
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
