//go:build unix

package upgrade

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

// registered tracks the live listeners eligible for handoff, keyed by a stable
// address key ("tcp:host:port" / "udp:host:port"). Guarded by mu.
var (
	mu         sync.Mutex
	registered = make(map[string]*os.File)
	order      []string // registration order, for stable child FD numbering
)

// inheritedFiles maps an address key to the *os.File adopted from the parent.
// Populated once at startup from the environment.
var (
	inheritOnce  sync.Once
	inheritedMap map[string]*os.File
)

// parseInherited reads the FD map the parent passed via the environment and
// reconstructs *os.File handles for each inherited socket. The parent places
// the sockets as extra files starting at fd 3, in the order listed in the env
// variable.
func parseInherited() map[string]*os.File {
	inheritOnce.Do(func() {
		inheritedMap = make(map[string]*os.File)
		spec := os.Getenv(envFDList)
		if spec == "" {
			return
		}
		keys := strings.Split(spec, ",")
		for i, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			// Extra files begin at fd 3 in the child.
			fd := uintptr(3 + i)
			f := os.NewFile(fd, key)
			if f == nil {
				continue
			}
			inheritedMap[key] = f
		}
	})
	return inheritedMap
}

func supported() bool { return true }

// adoptListener returns a net.Listener built from an inherited socket for key,
// or nil if none was inherited.
func adoptListener(key string) net.Listener {
	f := parseInherited()[key]
	if f == nil {
		return nil
	}
	l, err := net.FileListener(f)
	if err != nil {
		return nil
	}
	return l
}

// adoptPacketConn returns a *net.UDPConn built from an inherited socket for
// key, or nil if none was inherited.
func adoptPacketConn(key string) *net.UDPConn {
	f := parseInherited()[key]
	if f == nil {
		return nil
	}
	pc, err := net.FilePacketConn(f)
	if err != nil {
		return nil
	}
	uc, ok := pc.(*net.UDPConn)
	if !ok {
		_ = pc.Close()
		return nil
	}
	return uc
}

// registerFile records the dup'd socket file under key, in registration order,
// for a later handoff. A second registration for the same key replaces the
// first.
func registerFile(key string, f *os.File) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registered[key]; !exists {
		order = append(order, key)
	}
	registered[key] = f
}

func listenTCP(addr string) (net.Listener, error) {
	key := tcpKey(addr)

	// Adopt an inherited socket if the parent handed one over.
	if l := adoptListener(key); l != nil {
		if tl, ok := l.(*net.TCPListener); ok {
			if f, err := tl.File(); err == nil {
				registerFile(key, f)
			}
		}
		return l, nil
	}

	// Bind fresh and register for a future handoff.
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	if tl, ok := l.(*net.TCPListener); ok {
		if f, err := tl.File(); err == nil {
			registerFile(key, f)
		}
	}
	return l, nil
}

func listenUDP(addr string) (*net.UDPConn, error) {
	key := udpKey(addr)

	if uc := adoptPacketConn(key); uc != nil {
		if f, err := uc.File(); err == nil {
			registerFile(key, f)
		}
		return uc, nil
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	uc, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	if f, err := uc.File(); err == nil {
		registerFile(key, f)
	}
	return uc, nil
}

// perform spawns a child copy of this binary, passing the registered listening
// sockets as extra file descriptors and the address-key list via the
// environment. The child adopts the sockets and starts serving immediately.
func perform() (int, error) {
	mu.Lock()
	keys := append([]string(nil), order...)
	files := make([]*os.File, 0, len(keys))
	for _, k := range keys {
		files = append(files, registered[k])
	}
	mu.Unlock()

	if len(files) == 0 {
		return 0, errors.New("no listening sockets registered for handoff")
	}

	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("locate executable: %w", err)
	}

	// Build the child environment: inherit ours, then set/replace the FD map.
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, envFDList+"=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, envFDList+"="+strings.Join(keys, ","))

	attr := &os.ProcAttr{
		Dir: "",
		Env: env,
		// 0,1,2 are stdio; the registered sockets follow as fd 3, 4, ...
		Files: append([]*os.File{os.Stdin, os.Stdout, os.Stderr}, files...),
	}

	proc, err := os.StartProcess(exe, os.Args, attr)
	if err != nil {
		return 0, fmt.Errorf("start child: %w", err)
	}
	return proc.Pid, nil
}
