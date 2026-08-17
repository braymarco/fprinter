//go:build !windows

package main

import "fprinter/config"

// run para plataformas no-Windows: solo modo consola, sin SCM.
func run(cfg *config.Config) error {
	return newServer(cfg).start()
}
