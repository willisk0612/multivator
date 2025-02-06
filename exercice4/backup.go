package main

import "os/exec"

func run() {

	exec.Command("cmd", "/C", "start", "powershell", "go", "run", "primary.go").Run()

}
