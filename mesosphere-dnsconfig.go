package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

const debug = true
const prefix = "config"
const suffix = "_mesosphere."
const fsprefix = ""

var priority = make(map[string][]string)

func main() {
	service := flag.String("service", "", "service to configure: mesos-master, mesos-slave, marathon or zookeeper")
	hostname := flag.String("hostname", "", "hostname to use, os hostname is used by default")
	write := flag.Bool("write", false, "write configs to files")
	exec := flag.Bool("exec", false, "start service")
	flag.Parse()

	if *service == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *hostname == "" {
		host, err := os.Hostname()
		if err != nil {
			log.Fatal(err)
		}

		*hostname = host
	}

	dprint(fmt.Sprintf("using hostname %s", *hostname))

	priority["mesos-master"] = append(priority["mesos-master"], ".mesos-master.", ".mesos.")
	priority["mesos-slave"] = append(priority["mesos-slave"], ".mesos-slave.", ".mesos.")
	priority["marathon"] = append(priority["marathon"], ".marathon.", ".mesos.")
	priority["zookeeper"] = append(priority["zookeeper"], ".zookeeper.")
	_, exists := priority[*service]
	if exists == false {
		log.Fatalln(fmt.Sprintf("unknown service '%s'", *service))
	}

	options, flags := findConfig(*service, *hostname)

	if *write && *exec {
		commitConfig(*service, options, flags)
		restartService(*service)
	} else if *write {
		commitConfig(*service, options, flags)
	} else if *exec {
		runInForeground(*service, options, flags)
	}
}

func findConfig(service string, hostname string) (map[string]string, []string) {
	options := make(map[string]string)
	flags := make(map[string]bool)

	hostname_parts := strings.Split(hostname, ".")

	records := map[string][]string{}

	wg := sync.WaitGroup{}
	for i := range hostname_parts {
		domain := strings.Join(hostname_parts[i:], ".")

		for y := range priority[service] {
			dnsname := prefix + priority[service][y] + suffix + domain

			wg.Add(1)
			go func() {
				txt, err := net.LookupTXT(dnsname)
				if err != nil {
					dprint(fmt.Sprintf("%s", err))
				} else {
					records[dnsname] = txt
				}
				wg.Done()
			}()
		}
	}

	wg.Wait()

	// traverse through the hostname
	for i := range hostname_parts {
		domain := strings.Join(hostname_parts[i:], ".")

		// traverse through the services
		for y := range priority[service] {
			dnsname := prefix + priority[service][y] + suffix + domain

			txts := records[dnsname]

			// iterate all returned txt strings
			for t := range txts {

				s := strings.SplitN(txts[t], "=", 2)

				if len(s) == 1 {
					dprint(fmt.Sprintf("%s: enabling %s", dnsname, s[0]))
					flags[s[0]] = true
				} else if len(s) == 2 {
					current_value, exists := options[s[0]]
					if exists {
						dprint(fmt.Sprintf("option %s is already defined as %s, not overwriting with %s", s[0], current_value, s[1]))
					} else {
						// if we are dealing with marathon, we should derive zk and master options
						// from mesos config, if they were not passed to marathon directly
						if s[0] == "zk" && service == "marathon" && priority[service][y] == ".mesos." {
							// option is not set for marathon
							options["zk"] = strings.Replace(s[1], "/mesos", "/marathon", 1)

							if _, ok := options["master"]; !ok {
								options["master"] = s[1]
							}
						} else {
							dprint(fmt.Sprintf("%s %s: found %s => %s", dnsname, priority[service][y], s[0], s[1]))
							options[s[0]] = s[1]
						}
					}
				} else {
					dprint(fmt.Sprintf("unknown contents %s", s))
				}
			}
		}
	}

	f := make([]string, 0, len(flags))
	for flag := range flags {
		f = append(f, flag)
	}

	return options, f
}

func dprint(txt string) {
	if debug {
		log.Println(txt)
	}
}

func commitConfig(service string, options map[string]string, flags []string) {
	switch service {
	case "mesos":
		writeMesosphereConfig("/etc/mesos/", options, flags)
	case "mesos-master":
		writeMesosphereConfig("/etc/mesos-master/", options, flags)
	case "mesos-slave":
		writeMesosphereConfig("/etc/mesos-slave/", options, flags)
	case "marathon":
		writeMesosphereConfig("/etc/marathon/conf/", options, flags)
	case "zookeeper":
		writeZookeeperConfig("/var/lib/zookeeper/", "/etc/zookeeper/conf/", options)
	}
}

func restartService(service string) {
	cmd := exec.Command("service", service, "restart")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(out))
		log.Fatal(err)
	}
}

func runInForeground(service string, options map[string]string, flags []string) {
	var err error

	switch service {
	case "mesos-master", "mesos-slave", "marathon":
		log.Println("running:", service, strings.Join(mesosArgs(options, flags), " "))
		err = exec.Command(service, mesosArgs(options, flags)...).Run()
	case "zookeeper":
		log.Println("/usr/share/zookeeper/bin/zkServer.sh start-foreground")
		err = exec.Command("/usr/share/zookeeper/bin/zkServer.sh", "start-foreground").Run()
	}

	if err != nil {
		log.Fatal(err)
	}
}

func mesosArgs(options map[string]string, flags []string) []string {
	args := []string{}

	for k, v := range options {
		args = append(args, "--"+k+"="+v)
	}

	for _, f := range flags {
		args = append(args, "--"+f)
	}

	return args
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

func writeMesosphereConfig(output_directory string, options map[string]string, flags []string) {
	output_directory = fsprefix + output_directory

	err := os.MkdirAll(output_directory, 0755)
	if err != nil {
		log.Fatalln(err)
	}

	for option := range options {
		log.Printf("option: %s=%s\n", option, options[option])
		writeConfigFile(output_directory, option, []byte(options[option]+"\n"))
	}

	for _, flag := range flags {
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
			writeConfigFile(myid_dir, option, []byte(options[option]+"\n"))
		} else {
			zoocfg += option + "=" + options[option] + "\n"
		}
	}

	if len(zoocfg) > 0 {
		writeConfigFile(zoocfg_dir, "zoo.cfg", []byte(zoocfg))
	}
}
