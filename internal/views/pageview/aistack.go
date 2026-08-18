package pageview

import "fmt"

// AIStackRow returns the local-AI row text. accelerator names the compute
// stack selected for this machine's hardware ("CUDA", "ROCm", "CPU"), and
// accelerated is false when no GPU was found.
func AIStackRow(accelerator string, accelerated bool) Row {
	row := Row{Title: "Local AI Model Server"}
	if accelerated {
		row.Subtitle = fmt.Sprintf("Runs on this machine's %s hardware — nothing leaves the device", accelerator)
		return row
	}
	// A GPU-less host still gets a working stack, but saying so without
	// saying it is slow would set the wrong expectation for a first run.
	row.Subtitle = "No GPU detected — runs on the CPU, which is considerably slower"
	return row
}

// AIStackUnavailableSubtitle returns the subtitle for a host that cannot run
// the stack at all.
func AIStackUnavailableSubtitle() string {
	return "Podman is not installed, so there is nothing to run the container with"
}

// AIStackResultSubtitle returns the subtitle after the switch is toggled.
// port is the published API port.
func AIStackResultSubtitle(enabled bool, port int) string {
	if enabled {
		// The port is the whole point of the feature — without it the user
		// has a running container and no idea how to reach it.
		return fmt.Sprintf("Serving on http://localhost:%d — the first start downloads several GB", port)
	}
	return "Stopped and removed — the downloaded model was kept"
}
