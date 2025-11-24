# caddy-wakeonlan

A simple Caddy v2 HTTP handler that sends a Wake-On-LAN (WOL) magic packet to a specified MAC address at a given target IP whenever the site or route is visited.

**Works for networks without a broadcast address by sending the packet directly to the host (unicast WOL).**

## Features
- Caddy v2 HTTP middleware (handler directive)
- Unicast WOL to a specific IP
- Optional custom UDP port (defaults to 9)
- Non-blocking: requests proceed even if sending the packet fails

## Build
```
xcaddy build --with github.com/bartosz-kakol/caddy-wakeonlan
```

## Usage (Caddyfile)
Directive syntax: `wake_on_lan <mac> <ip-or-host> [port]`

Example site block:
```Caddyfile
www.example.com {
    wake_on_lan 10:ff:e0:cf:e6:0e 123.123.1.3
    # optional port override (defaults to 9):
    # wake_on_lan 10:ff:e0:cf:e6:0e 123.123.1.3 9
    
    reverse_proxy http://123.123.1.3:3923
}
```

## Notes
- Supported MAC formats: `aa:bb:cc:dd:ee:ff`, `aa-bb-cc-dd-ee-ff`, or `aabbccddeeff`
- If ip-or-host is a hostname, it is resolved at runtime
- Errors while sending the packet are ignored so they don’t impact the HTTP response path
