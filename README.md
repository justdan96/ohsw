# Oh! Service Wrapper (ohsw)

> A simple service wrapper written in Golang, initially for controlling OHS on Windows

## Purpose

This wrapper is for controlling programs that do not support being run as a Windows service natively. The initial implementation supports controlling OHS. Configuration is handled in a JSON file. It is written in Golang, so compilation is simply a process of running `go build` and waiting for the executable to be compiled. There are no external dependencies.

## Releases

Please check the "Releases" tab on GitHub for the latest release.

## Installation

First you must configure ohsw.json. The file in this repository can serve as an example. If you are wrapping OHS, you simply have to change the paths. 

Next make sure that your OHS installation has saved credentials for the user that will run the service. You can store them using this command:

```
E:\Oracle\Middleware\user_projects\FND1\httpConfig\ohs\bin\startComponent.cmd ohs1 storeUserConfig
```

Now you are ready to install the service! Copy ohsw.exe and ohsw.json to somewhere on the server. Perhaps to `E:\Oracle\Middleware\user_projects\FND1\httpConfig\ohs\bin\`. Then open a command prompt and run:

```
ohsw.exe -service install
```

Check in services.msc and you should have a brand new OHS service! Just make sure that OHS isn't already started and then start as you usually would.

## Usage

When you use the start and stop actions in the service panel it will run the corresponding scripts in StartArgs and StopArgs of the JSON configuration file. The PidFile entry is important, as it needs to read the PID of OHS from this file. Every 30 seconds it will check that this PID still exists.

The logging of OHSW is all done into the Windows Event Log. The stdout and stderr streams of the start and stop scripts will spool to the files specified as Stdout and Stderr in the config file.

Please note that the wrapper checks the status of OHS and runs scripts when Windows service actions are invoked. If OHSW crashes for any reason the Windows service will show as stopped but OHS will still be running. Use at your own risk!

## Compilation

Any recent version of Go should work. The project will only work on Windows. You should be able to `go get` it, otherwise for manual compilation:

```
git clone github.com/justdan96/ohsw
cd ohsw
go get github.com/kardianos/service
go get github.com/mitchellh/go-ps
go build ohsw.go
```

This will produce `ohsw.exe` in the current folder.

## Credits

Program written by Daniel Bryant.
