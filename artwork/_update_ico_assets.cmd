@echo off
del /q ..\assets\*.ico
for %%F in ("*.png") do (
    \bin\go-png2ico\go-png2ico %%~nxF ..\assets\%%~nF.ico
)
