// Command crabby-windows is the Windows entrypoint. Build it as crabby.exe.
//
// It does nothing but forward the command into WSL:
//
//	crabby.exe init   ->   wsl bash -lc "cd <wslpath> && crabby init"
package main

import (
	"os"

	"github.com/marioolf/crabby/internal/windows"
)

func main() {
	os.Exit(windows.Forward(os.Args[1:]))
}
