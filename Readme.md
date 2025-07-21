Proxmox VDI Client
==================

https://github.com/joshpatten/PVE-VDIClient

Usage
-----
```
Usage of proxmox-vdiclient [flags] [vmid/name [operation]]:

  operations: status, start, stop, reset, open (default)

  -autostart-vm
    	start stopped VMs before opening (default true)
  -config value
    	Path to config file (default ~/.config/proxmox-vdiclient/proxmox-vdiclient.conf)
  -fullscreen
    	fullscreen mode
  -host string
    	proxmox hostname
  -kiosk
    	kiosk mode
  -override-theme string
    	override theme (dark/light)
  -port int
    	proxmox port (default 8006)
  -refresh-interval duration
    	refresh interval (default 30s)
  -remote-viewer string
    	remote viewer executable (default "remote-viewer")
  -title string
    	title shown in gui (default "proxmox-vdiclient")
  -token-name string
    	token name
  -token-value string
    	token value
  -unsafe-skip-tls-verify
    	skip TLS certificate verification
```

Configuration file
------------------
The configuration file is a simple file with one option per line. Comment lines with '#' are ignored.

Commandline options can be set from the configuration file.
The default configuration file is loaded before options are processed.

Example:
```
title = Available VMs on your Proxmox
host = hostname.proxmox.server
# start remote-viewer in fullscreen mode
fullscreen = true
```

Permissions
-----------

| Permission   | needed for                     |
|--------------|--------------------------------|
| VM.Audit     | list and query VM              |
| VM.Console   | open SPICE session             |
| VM.PowerMgmt | start/stop/reset VM (optional) |
