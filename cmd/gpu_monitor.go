package cmd

import (
	"os"
	"os/exec"
)

// gpuMonitor wraps a spawned terminal running a GPU stats tool (nvtop,
// nvidia-smi, radeontop, intel_gpu_top). Silent no-op if prerequisites
// aren't met — never errors into the user's run output.
type gpuMonitor struct {
	cmd *exec.Cmd
}

// startGPUMonitor pops up a new terminal window with the best available
// GPU monitoring tool. Returns nil if any step fails: no GPU, no monitor
// binary, no supported terminal emulator, non-interactive shell, etc.
func startGPUMonitor() *gpuMonitor {
	if flagNoGPU {
		return nil
	}
	if !isInteractiveDisplay() {
		return nil
	}

	monitor := pickGPUMonitor()
	if monitor == "" {
		return nil
	}
	cmd := spawnInTerminal(monitor)
	if cmd == nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}
	return &gpuMonitor{cmd: cmd}
}

func (m *gpuMonitor) stop() {
	if m == nil || m.cmd == nil || m.cmd.Process == nil {
		return
	}
	_ = m.cmd.Process.Kill()
	_ = m.cmd.Wait()
}

func pickGPUMonitor() string {
	if hasBin("nvidia-smi") {
		if hasBin("nvtop") {
			return "nvtop"
		}
		return "nvidia-smi -l 1"
	}
	if hasBin("nvtop") {
		return "nvtop" // covers AMD/Intel too via libdrm
	}
	if hasBin("radeontop") {
		return "radeontop"
	}
	if hasBin("intel_gpu_top") {
		return "sudo -n intel_gpu_top" // typically needs root; -n exits if no cached sudo
	}
	return ""
}

func hasBin(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// spawnInTerminal finds a terminal emulator and returns an *exec.Cmd that
// opens a new window running cmdline. Nil if no supported terminal found.
func spawnInTerminal(cmdline string) *exec.Cmd {
	type spec struct {
		bin  string
		args []string
	}
	// Ordered by preference: lightweight modern emulators first.
	specs := []spec{
		{"kitty", []string{"--title=lookout-gpu", "sh", "-c", cmdline}},
		{"wezterm", []string{"start", "--", "sh", "-c", cmdline}},
		{"alacritty", []string{"-t", "lookout-gpu", "-e", "sh", "-c", cmdline}},
		{"ghostty", []string{"-e", cmdline}},
		{"foot", []string{"-T", "lookout-gpu", "sh", "-c", cmdline}},
		{"gnome-terminal", []string{"--title=lookout-gpu", "--", "sh", "-c", cmdline}},
		{"konsole", []string{"-e", "sh", "-c", cmdline}},
		{"tilix", []string{"-e", cmdline}},
		{"xfce4-terminal", []string{"--title=lookout-gpu", "-e", cmdline}},
		{"terminator", []string{"-T", "lookout-gpu", "-e", cmdline}},
		{"xterm", []string{"-title", "lookout-gpu", "-e", "sh", "-c", cmdline}},
	}
	// Env var escape hatch for unusual setups.
	if t := os.Getenv("LOOKOUT_TERMINAL"); t != "" {
		specs = append([]spec{{t, []string{"-e", "sh", "-c", cmdline}}}, specs...)
	}
	for _, s := range specs {
		if !hasBin(s.bin) {
			continue
		}
		return exec.Command(s.bin, s.args...)
	}
	return nil
}
