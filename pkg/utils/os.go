//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris || windows

package utils

import "runtime"

// PrettierOSName provides a prettier OS name
func PrettierOSName() string {
	switch runtime.GOOS {
	case "aix":
		return "AIX"
	case "darwin":
		return "Darwin"
	case "dragonfly":
		return "DragonFly"
	case "freebsd":
		return "FreeBSD"
	case "illumos":
		return "Illumos"
	case "linux":
		return "Linux"
	case "netbsd":
		return "NetBSD"
	case "openbsd":
		return "OpenBSD"
	// Not support plan9 due to github.com/fasthttp/tcplisten
	//case "plan9":
	//	return "Plan9"
	case "solaris":
		return "Solaris"
	case "windows":
		return "Windows"
	default:
		return "Unknown"
	}
}
