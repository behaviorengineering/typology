package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

// findCableBoardSrc locates viewer/cable-board for boards serve --dev.
// Order: explicit flag, TYPOLOGY_VIEWER_SRC, walk from cwd (incl. providers/typology/...).
func findCableBoardSrc(explicit string) (string, error) {
	if s := strings.TrimSpace(explicit); s != "" {
		return validateCableBoardSrc(s)
	}
	if s := strings.TrimSpace(os.Getenv("TYPOLOGY_VIEWER_SRC")); s != "" {
		return validateCableBoardSrc(s)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("boards serve --dev: cwd: %w", err)
	}
	candidates := []string{
		filepath.Join(cwd, "viewer", "cable-board"),
		filepath.Join(cwd, "providers", "typology", "viewer", "cable-board"),
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		candidates = append(candidates,
			filepath.Join(dir, "viewer", "cable-board"),
			filepath.Join(dir, "providers", "typology", "viewer", "cable-board"),
		)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		if _, err := validateCableBoardSrc(abs); err == nil {
			return abs, nil
		}
	}
	return "", fmt.Errorf("boards serve --dev: cannot find viewer/cable-board (pass --viewer-src or set TYPOLOGY_VIEWER_SRC); need a Typology checkout with npm sources")
}

func validateCableBoardSrc(dir string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return "", err
	}
	pkg := filepath.Join(abs, "package.json")
	if _, err := os.Stat(pkg); err != nil {
		return "", fmt.Errorf("boards serve --dev: %s is not a cable-board app (missing package.json)", abs)
	}
	return abs, nil
}

func splitServeHostPort(addr string) (host, port string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1", "5173"
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1", strings.TrimPrefix(addr, ":")
	}
	h, p, err := net.SplitHostPort(addr)
	if err != nil || p == "" {
		return "127.0.0.1", "5173"
	}
	if h == "" || h == "0.0.0.0" {
		h = "127.0.0.1"
	}
	return h, p
}

func ensureNpmInstall(viewerSrc string, stdout, stderr io.Writer) error {
	if _, err := os.Stat(filepath.Join(viewerSrc, "node_modules")); err == nil {
		return nil
	}
	_, _ = fmt.Fprintf(stdout, "boards serve --dev: npm install in %s\n", viewerSrc)
	cmd := exec.Command("npm", "install")
	cmd.Dir = viewerSrc
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// runBoardsServeDev starts Vite against viewerPublic (XDG boards) with HMR.
// It always runs in the foreground: stdout/stderr stay attached and the process
// blocks until Vite exits (Ctrl+C). It never daemonizes.
func runBoardsServeDev(addr, viewerPublic, viewerSrc string, stdout, stderr io.Writer) int {
	src, err := findCableBoardSrc(viewerSrc)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := ensureNpmInstall(src, stdout, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "boards serve --dev: npm install: %v\n", err)
		return 1
	}
	host, port := splitServeHostPort(addr)
	urlHost := host + ":" + port

	cmd := exec.Command("npm", "run", "dev", "--", "--host", host, "--port", port, "--strictPort")
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "TYPOLOGY_VIEWER_PUBLIC="+viewerPublic)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	setServeForegroundProcGroup(cmd)

	_, _ = fmt.Fprintf(stdout, "boards serve --dev: Vite in %s\n", src)
	_, _ = fmt.Fprintf(stdout, "boards serve --dev: publicDir=%s\n", viewerPublic)
	_, _ = fmt.Fprintf(stdout, "boards serve --dev: http://%s/  (foreground HMR; Ctrl+C to stop)\n", urlHost)
	_, _ = fmt.Fprintf(stdout, "boards serve --dev: open http://%s/?board=<id>\n", urlHost)

	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(stderr, "boards serve --dev: start: %v\n", err)
		return 1
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigc
		signal.Stop(sigc)
		interruptServeChild(cmd, sig)
	}()

	err = cmd.Wait()
	signal.Stop(sigc)
	if err != nil {
		if isServeInterrupt(err) {
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "boards serve --dev: %v\n", err)
		return 1
	}
	return 0
}
