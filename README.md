# pap2t-unlock

A tool for unlocking Cisco/Linksys PAP2T VoIP adapters by running a fake provisioning environment (DHCP, DNS, and HTTP servers) on a local network interface. When the PAP2T connects, it receives an IP via DHCP, all DNS queries resolve to the server, and the HTTP server serves a configuration profile that clears passwords and disables provisioning.

**This tool is for demonstration purposes only. Use at your own risk.**

## Download

Download the latest binary for your architecture from the [GitHub Releases page](https://github.com/dimitrovs/pap2t-unlock/releases).

- `pap2t-unlock` — Linux amd64
- `pap2t-unlock-arm64` — Linux arm64

After downloading, make it executable:

```bash
chmod +x pap2t-unlock
```

## Usage

### Prerequisites

- A Linux machine with an available ethernet interface (e.g. `eth0`)
- Root/sudo access (the tool binds to ports 67, 53, and 80)
- **Important:** If you are connected to this machine over SSH, make sure you are connected over WiFi, not ethernet. This tool will take over the ethernet interface and you will lose your SSH connection if it is over ethernet.

### Steps

1. Run the tool with sudo, specifying your ethernet interface:

   ```bash
   sudo ./pap2t-unlock eth0
   ```

2. Plug the PAP2T into the ethernet port and power it on.

3. Wait until you see the PAP2T load the `.cfg` file in the tool's log output. Any attempts to connect to SIP should stop shortly after that.

4. Unplug the PAP2T from the ethernet port and from power.

5. Perform a phone reset:
   - Pick up the handset connected to the **Phone 1** port.
   - Dial `****` (four asterisks) to enter the configuration menu.
   - Dial `73738#` (R-E-S-E-T-#).
   - Press `1` to confirm.

The PAP2T should now be factory reset with provisioning disabled.
