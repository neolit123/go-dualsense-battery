@echo off

echo enabling CGO...
set OLD_CGO_ENABLED=%CGO_ENABLED%
set CGO_ENABLED=1

for /f %%a in ('git describe --tags') do set "BUILD_VERSION=%%a"
echo using BUILD_VERSION %BUILD_VERSION%

echo calling go-windres...
go-winres simply --icon .\assets\charging_3.ico --manifest gui

echo building...
go build -ldflags "-s -H=windowsgui -X=main.version=%BUILD_VERSION%"

echo generating SHA256...
sha256sum go-dualsense-battery.exe > go-dualsense-battery.exe.sha256

set BUILD_VERSION=
set CGO_ENABLED=%OLD_CGO_ENABLED%
