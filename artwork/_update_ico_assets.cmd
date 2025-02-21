@echo off
del /q ..\assets\*.ico
for %%F in ("*.png") do (
    go-png2ico %%~nxF ..\assets\%%~nF.ico
)
