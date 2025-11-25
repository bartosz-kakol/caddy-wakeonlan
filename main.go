package caddy_wakeonlan

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

// WakeOnLAN is an HTTP middleware handler that sends a Wake-On-LAN magic packet
// to the specified IP for the given MAC address whenever the handler is invoked.
//
// Example Caddyfile usage:
//
//	wake_on_lan <mac> <ip> [port]
//
// If port is omitted, UDP/9 is used by default.
type WakeOnLAN struct {
	MAC  string `json:"mac,omitempty"`
	IP   string `json:"ip,omitempty"`
	Port int    `json:"port,omitempty"`
}

// CaddyModule returns the Caddy module information.
func (WakeOnLAN) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.wake_on_lan",
		New: func() caddy.Module { return new(WakeOnLAN) },
	}
}

// Validate ensures the configuration is sane.
func (w *WakeOnLAN) Validate() error {
	if w.MAC == "" {
		return errors.New("wake_on_lan: MAC must be specified")
	}
	if _, err := parseMAC(w.MAC); err != nil {
		return fmt.Errorf("wake_on_lan: invalid MAC %q: %w", w.MAC, err)
	}
	if w.IP == "" {
		return errors.New("wake_on_lan: IP must be specified")
	}
	if net.ParseIP(w.IP) == nil {
		// Allow hostnames too, as ResolveUDPAddr will handle those at runtime
		if _, err := net.ResolveUDPAddr("udp", net.JoinHostPort(w.IP, strconv.Itoa(w.portOrDefault()))); err != nil {
			return fmt.Errorf("wake_on_lan: invalid IP/host %q: %w", w.IP, err)
		}
	}
	if w.Port < 0 || w.Port > 65535 {
		return fmt.Errorf("wake_on_lan: invalid port %d", w.Port)
	}
	return nil
}

func (w *WakeOnLAN) portOrDefault() int {
	if w.Port == 0 {
		return 9
	}
	return w.Port
}

// ServeHTTP sends the WOL magic packet, then calls the next handler in the chain.
func (w *WakeOnLAN) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// Best-effort; don't block the request if sending fails.
	_ = sendWOL(w.MAC, w.IP, w.portOrDefault())
	return next.ServeHTTP(rw, r)
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
func (w *WakeOnLAN) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		args := d.RemainingArgs()
		if len(args) < 2 || len(args) > 3 {
			return d.ArgErr()
		}
		w.MAC = args[0]
		w.IP = args[1]
		w.Port = 0
		if len(args) == 3 {
			p, err := strconv.Atoi(args[2])
			if err != nil {
				return d.Errf("invalid port %q: %v", args[2], err)
			}
			w.Port = p
		}
		// No nested block expected
		if d.NextBlock(0) {
			return d.ArgErr()
		}
	}
	return nil
}

// Interface guards
var (
	_ caddy.Module                = (*WakeOnLAN)(nil)
	_ caddyhttp.MiddlewareHandler = (*WakeOnLAN)(nil)
	_ caddyfile.Unmarshaler       = (*WakeOnLAN)(nil)
)

func init() {
	caddy.RegisterModule(WakeOnLAN{})
	httpcaddyfile.RegisterHandlerDirective("wake_on_lan", func(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
		var w WakeOnLAN
		if err := w.UnmarshalCaddyfile(h.Dispenser); err != nil {
			return nil, err
		}
		if err := w.Validate(); err != nil {
			return nil, err
		}
		return &w, nil
	})
}

// parseMAC parses MAC addresses in common formats (with ':' or '-' separators, or raw hex).
func parseMAC(s string) (net.HardwareAddr, error) {
	// Try standard parser first
	if hw, err := net.ParseMAC(s); err == nil {
		return hw, nil
	}
	// Remove common separators and try again as raw hex
	cleaned := strings.ReplaceAll(strings.ReplaceAll(s, ":", ""), "-", "")
	if len(cleaned) != 12 {
		return nil, fmt.Errorf("unexpected MAC length after cleanup: %d", len(cleaned))
	}
	b := make([]byte, 0, 6)
	for i := 0; i < 12; i += 2 {
		v, err := strconv.ParseUint(cleaned[i:i+2], 16, 8)
		if err != nil {
			return nil, err
		}
		b = append(b, byte(v))
	}
	return net.HardwareAddr(b), nil
}

func sendWOL(macStr, ip string, port int) error {
	hw, err := parseMAC(macStr)
	if err != nil {
		return err
	}

	// Build magic packet: 6 x 0xFF followed by MAC repeated 16 times
	packet := make([]byte, 6+16*6)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], hw)
	}

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	return err
}
