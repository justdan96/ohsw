// Copyright 2020 Daniel Bryant
// This work is licensed under a Creative Commons Attribution-NonCommercial-ShareAlike 4.0 International License
// Attribution must be retained by keeping this copyright and license notice on any derived works

// Oh! Service Wrapper
package main

import (
  "encoding/json"
  "flag"
  "fmt"
  "time"
  "log"
  "os"
  "os/exec"
  "path/filepath"
  "io/ioutil"
  "strings"
  "strconv"

  "github.com/kardianos/service"
  "github.com/mitchellh/go-ps"
)

// Config is the structure of the config file
type Config struct {
  Name, DisplayName, Description string

  Dir  string
  Exec string
  StartArgs []string
  StopArgs []string
  Env  []string
  PidFile string
  Dependency string
  
  Stderr, Stdout string
}

var logger service.Logger

type program struct {
  exit    chan struct{}
  service service.Service

  *Config

  startCmd *exec.Cmd
  stopCmd *exec.Cmd
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(filename string) bool {
    info, err := os.Stat(filename)
    if os.IsNotExist(err) {
        return false
    }
    return !info.IsDir()
}

func (p *program) Start(s service.Service) error {
  // Look for exec.
  // Verify home directory.
  fullExec, err := exec.LookPath(p.Exec)
  if err != nil {
    return fmt.Errorf("Failed to find executable %q: %v", p.Exec, err)
  }

  // build the command line for the script and accompanying environment
  p.startCmd = exec.Command(fullExec, p.StartArgs...)
  p.startCmd.Dir = p.Dir
  p.startCmd.Env = append(os.Environ(), p.Env...)

  // call the actually service starting in a subroutine, so Start is non-blocking
  go p.run()
  return nil
}

func (p *program) run() error {
  logger.Info("Starting ", p.DisplayName)
  // if this function ever returns it is because something went wrong - so we should exit the program
  defer os.Exit(1)

  // open the stdout and stderr files, we will attach the stdout and stderr streams of our script to these files
  if p.Stderr != "" {
    f, err := os.OpenFile(p.Stderr, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0777)
    if err != nil {
      logger.Errorf("Failed to open std err %q: %v", p.Stderr, err)
      return fmt.Errorf("Failed to open std err %q: %v", p.Stderr, err)
    }
    defer f.Close()
    p.startCmd.Stderr = f
  }
  if p.Stdout != "" {
    f, err := os.OpenFile(p.Stdout, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0777)
    if err != nil {
      logger.Errorf("Failed to open std out %q: %v", p.Stdout, err)
      return fmt.Errorf("Failed to open std out %q: %v", p.Stdout, err)
    }
    defer f.Close()
    p.startCmd.Stdout = f
  }

  // try to run the script
  err := p.startCmd.Run()
  if err != nil {
    logger.Errorf("Error running: %v", err)
    return fmt.Errorf("Error running: %v", err)
  }

  // wait on the PID file to exist, time out after a number of tries
  tries := 5
  try := 0
  for !fileExists(p.PidFile) && try < tries {
    time.Sleep(5 * time.Second)
    try += 1
  }
  
  // handle failure to find PID file
  if !fileExists(p.PidFile) && try >= tries {
    logger.Errorf("Error, timed out waiting on PID file: %v, tried %v times", p.PidFile, tries)
    return fmt.Errorf("Error, timed out waiting on PID file: %v, tried %v times", p.PidFile, tries)
  // if the PID file does exist, try to open and read the PID from that file and then check that a process with that PID is running
  } else if fileExists(p.PidFile) {
    // read the service PID out of the PID file
    pid_b, err := ioutil.ReadFile(p.PidFile)
    if err != nil {
      logger.Errorf("Error reading PID file: %v", p.PidFile)
      return fmt.Errorf("Error reading PID file: %v", p.PidFile)
    } else {
      // convert PID to string, trim all spaces and then convert to integer
      // pib_b = byte array, pid_s = string, pid = integer
      pid_s := strings.TrimSpace(string(pid_b))
      pid, _ := strconv.Atoi(string(pid_s))
      // we can't use os.FindProcess as it won't report if a process was killed
      // we use ps.FindProcess - pcs will be nil and error will be nil if a matching process is not found
      pcs, _ := ps.FindProcess(pid)
      if pcs == nil {
        logger.Errorf("Error searching for process with PID: %v", pid)
        return fmt.Errorf("Error searching for process with PID: %v", pid)
      } else {
        logger.Infof("Component successfully started with PID: %v", pid)
        
        // we have to catch OHS in this loop, check for it every 30 seconds
        for pcs != nil {
          // commented out as this makes the event log for OHS _very_ chatty
          // logger.Infof("Component successfully pinged PID: %v", pid)
          time.Sleep(30 * time.Second)
          pcs, _ = ps.FindProcess(pid)
        }
        
        // if we got to this point the loop above has been broken
        logger.Errorf("Component with PID %v unexpectedly died!", pid)
        return fmt.Errorf("Component with PID %v unexpectedly died!", pid)
      }
    }
  }
  
  // pretty sure this will never call...
  return nil
}

func (p *program) kill() error {
  logger.Info("Killing ", p.DisplayName)

  if p.Stderr != "" {
    f, err := os.OpenFile(p.Stderr, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0777)
    if err != nil {
      logger.Errorf("Failed to open std err %q: %v", p.Stderr, err)
      return fmt.Errorf("Failed to open std err %q: %v", p.Stderr, err)
    }
    defer f.Close()
    p.stopCmd.Stderr = f
  }
  if p.Stdout != "" {
    f, err := os.OpenFile(p.Stdout, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0777)
    if err != nil {
      logger.Errorf("Failed to open std out %q: %v", p.Stdout, err)
      return fmt.Errorf("Failed to open std out %q: %v", p.Stdout, err)
    }
    defer f.Close()
    p.stopCmd.Stdout = f
  }

  err := p.stopCmd.Run()
  if err != nil {
    logger.Errorf("Error running: %v", err)
    return fmt.Errorf("Error running: %v", err)
  }
  
  // wait on the PID file to NOT exist, timeout after 
  tries := 5
  try := 0
  for fileExists(p.PidFile) && try < tries {
    time.Sleep(5 * time.Second)
    try += 1
  }
  
  // handle PID file still existing after timeout
  if fileExists(p.PidFile) && try >= tries {
    logger.Errorf("Error, timed out waiting on PID file to delete: %v, tried %v times", p.PidFile, tries)
    return fmt.Errorf("Error, timed out waiting on PID file to delete: %v, tried %v times", p.PidFile, tries)
  }

  return nil
}

func (p *program) Stop(s service.Service) error {
  // Look for exec.
  // Verify home directory.
  fullExec, err := exec.LookPath(p.Exec)
  if err != nil {
    return fmt.Errorf("Failed to find executable %q: %v", p.Exec, err)
  }

  // build the execution environment where we will run our stop script
  p.stopCmd = exec.Command(fullExec, p.StopArgs...)
  p.stopCmd.Dir = p.Dir
  p.stopCmd.Env = append(os.Environ(), p.Env...)

  // this is now a blocking operation...
  p.kill()
  return nil
}

func getConfigPath() (string, error) {
  fullexecpath, err := os.Executable()
  if err != nil {
    return "", err
  }

  dir, execname := filepath.Split(fullexecpath)
  ext := filepath.Ext(execname)
  name := execname[:len(execname)-len(ext)]

  return filepath.Join(dir, name+".json"), nil
}

func getConfig(path string) (*Config, error) {
  f, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  defer f.Close()

  conf := &Config{}

  r := json.NewDecoder(f)
  err = r.Decode(&conf)
  if err != nil {
    return nil, err
  }
  return conf, nil
}

func main() {
  // give us a way to install, uninstall, stop and start the service via the "-service" parameter
  svcFlag := flag.String("service", "", "Control the system service.")
  flag.Parse()

  // read the config file
  configPath, err := getConfigPath()
  if err != nil {
    log.Fatal(err)
  }
  config, err := getConfig(configPath)
  if err != nil {
    log.Fatal(err)
  }

  // build the definition of the Windows service
  svcConfig := &service.Config{
    Name:        config.Name,
    DisplayName: config.DisplayName,
    Description: config.Description,
    Dependencies: []string{config.Dependency},
  }
  
  // create program configuration and log errors
  prg := &program{
    exit: make(chan struct{}),

    Config: config,
  }
  s, err := service.New(prg, svcConfig)
  if err != nil {
    log.Fatal(err)
  }
  prg.service = s

  errs := make(chan error, 5)
  logger, err = s.Logger(errs)
  if err != nil {
    log.Fatal(err)
  }

  go func() {
    for {
      err := <-errs
      if err != nil {
        log.Print(err)
      }
    }
  }()

  // pass the service control actions along, if they are valid
  if len(*svcFlag) != 0 {
    err := service.Control(s, *svcFlag)
    if err != nil {
      log.Printf("Valid actions: %q\n", service.ControlAction)
      log.Fatal(err)
    }
    return
  }
  err = s.Run()
  if err != nil {
    logger.Error(err)
  }
}
