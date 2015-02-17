package main

import (
  "os"
  "net"
  "fmt"
  "log"
  "sort"
  "strings"
  "io/ioutil"
)

const debug    = true
const prefix   = "config"
const suffix   = "_mesosphere."
const fsprefix = ""
var priority   = make(map[string][]string)

func main() {

  if len(os.Args) < 2 || len(os.Args) > 3 {
    fmt.Printf("Usage: %s <service> [hostname]\n", os.Args[0])
    fmt.Println("<service> is one of mesos, mesos-master, mesos-slave, marathon or zookeeper")
    os.Exit(1)
  }

  service  := os.Args[1]
  hostname, err := os.Hostname()
  if len(os.Args) == 3 {
    hostname = os.Args[2]
  } else if err != nil {
    log.Fatalln(fmt.Sprintf("couldn't determine hostname: %s", err))
  }
  dprint(fmt.Sprintf("using hostname %s", hostname))

  priority["mesos"]        = append(priority["mesos"], ".mesos.")
//  priority["mesos"]        = append(priority["mesos"], ".")
  priority["mesos-master"] = append(priority["mesos-master"], ".mesos-master.")
//  priority["mesos-master"] = append(priority["mesos-master"], ".")
  priority["mesos-slave"]  = append(priority["mesos-slave"], ".mesos-slave.")
//  priority["mesos-slave"]  = append(priority["mesos-slave"], ".")
  priority["marathon"]     = append(priority["marathon"], ".marathon.")
//  priority["marathon"]     = append(priority["marathon"], ".")
  priority["zookeeper"]    = append(priority["zookeeper"], ".zookeeper.")
//  priority["zookeeper"]    = append(priority["zookeeper"], ".")
  _, exists := priority[service]
  if exists == false {
    log.Fatalln(fmt.Sprintf("unknown service '%s'", service))
  }

  options, flags := findConfig(service, hostname)
  commitConfig(service, options, flags)
}

func findConfig(service string, hostname string) (map[string]string, map[string]bool) {
  options := make(map[string]string)
  flags := make(map[string]bool)

  hostname_parts := strings.Split(hostname, ".")

  // traverse through the hostname
  for i := range hostname_parts {
    domain := strings.Join(hostname_parts[i:], ".")
    
    // traverse through the services
    for y := range priority[service] {
      dnsname := prefix + priority[service][y] + suffix + domain

        txts, err := net.LookupTXT(dnsname)
        if err != nil {
          dprint(fmt.Sprintf("%s", err))
          continue
        }

        // iterate all returned txt strings
        for t := range txts {

          s := strings.SplitN(txts[t], "=", 2)

          if len(s) == 1 {
            dprint(fmt.Sprintf("enabling %s", s[0]))
            flags[s[0]] = true
          } else if len(s) == 2 {
            current_value, exists := options[s[0]]
            if exists {
              dprint(fmt.Sprintf("option %s is already defined as %s, not overwriting with %s", s[0], current_value, s[1]))
            } else {
              dprint(fmt.Sprintf("found %s => %s", s[0], s[1]))
              options[s[0]] = s[1]
            }
          } else {
            dprint(fmt.Sprintf("unknown contents %s", s))
          }
          
        }
        
    }
  }

  return options, flags
}

func dprint(txt string) {
  if debug {
    log.Println(txt)
  }
}

func commitConfig(service string, options map[string]string, flags map[string]bool) {
  switch service {
    case "mesos": writeMesosphereConfig("/etc/mesos/", options, flags)
    case "mesos-master": writeMesosphereConfig("/etc/mesos-master/", options, flags)
    case "mesos-slave": writeMesosphereConfig("/etc/mesos-slave/", options, flags)
    case "marathon": writeMesosphereConfig("/etc/marathon/conf/", options, flags)
    case "zookeeper": writeZookeeperConfig("/var/lib/zookeeper/", "/etc/zookeeper/", options)
  }
}

func writeConfigFile(output_directory string, option string, data []byte) {

    output_file := output_directory + option
    dprint(fmt.Sprintf("writing %s", output_file))

    file, err := ioutil.TempFile(output_directory, ".mesospherednsconfig")
    if err != nil {
      log.Fatalln(err)
    }
    _, err = file.Write(data)
    if err != nil {
      log.Fatalln(err)
    }
    err = file.Close()
    if err != nil {
      log.Fatalln(err)
    }
    err = os.Chmod(file.Name(), 0644)
    if err != nil {
      log.Fatalln(err)
    }
    err = os.Rename(file.Name(), output_file)
    if err != nil {
      log.Fatalln(err)
    }
}

func writeMesosphereConfig(output_directory string, options map[string]string, flags map[string]bool) {
  output_directory = fsprefix + output_directory

  err := os.MkdirAll(output_directory, 0755)
  if err != nil {
    log.Fatalln(err)
  }

  for option := range options {
    log.Printf("option: %s=%s\n", option, options[option])
    writeConfigFile(output_directory, option, []byte(options[option] + "\n"))
  }

  for flag := range flags {
    log.Printf("flag: %s\n", flag)
    writeConfigFile(output_directory, "?"+flag, []byte(""))
  }
}

func writeZookeeperConfig(myid_dir string, zoocfg_dir string, options map[string]string) {
  myid_dir = fsprefix + myid_dir
  zoocfg_dir = fsprefix + zoocfg_dir

  err := os.MkdirAll(myid_dir, 0755)
  if err != nil {
    log.Fatalln(err)
  }
  err = os.MkdirAll(zoocfg_dir, 0755)
  if err != nil {
    log.Fatalln(err)
  }

  zoocfg := ""

  options_keys := make([]string, len(options))
  i := 0
  for k, _ := range options {
      options_keys[i] = k
      i++
  }
  sort.Strings(options_keys)

  for i := range options_keys {
    option := options_keys[i]
    log.Printf("option: %s=%s\n", option, options[option])
    if option == "myid" {
      writeConfigFile(myid_dir, option, []byte(options[option] + "\n"))
    } else {
      zoocfg += option + "=" + options[option] + "\n"
    }
  }

  if len(zoocfg) > 0 {
    writeConfigFile(zoocfg_dir, "zoo.cfg", []byte(zoocfg))
  }
}
