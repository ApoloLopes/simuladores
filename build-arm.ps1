# Compila os simuladores para Linux ARMv7 (binário estático).
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "arm"
$env:GOARM = "7"
go build -ldflags "-s -w" -o dist\simuladores-linux-armv7 .
