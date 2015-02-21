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
const fsprefix = "" // set to e.g. /tmp for debugging purposes

var nsprio = map[string][]string{}

func main() {
	service := flag.String("service", "", "service to configure: mesos-master, mesos-slave, marathon or zookeeper")
	hostname := flag.String("hostname", "", "hostname to use, os hostname is used by default")
	write := flag.Bool("write", false, "write config to files")
	exec := flag.Bool("exec", false, "(re)start service")
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

	nsprio["mesos-master"] = append(nsprio["mesos-master"], ".mesos-master.", ".mesos.")
	nsprio["mesos-slave"] = append(nsprio["mesos-slave"], ".mesos-slave.", ".mesos.")
	nsprio["marathon"] = append(nsprio["marathon"], ".marathon.", ".mesos.")
	nsprio["zookeeper"] = append(nsprio["zookeeper"], ".zookeeper.")
	if _, ok := nsprio[*service]; !ok {
		log.Fatalln(fmt.Sprintf("unknown service '%s'", *service))
	}

	options, flags := findConfig(*service, *hostname)

	if *write && *exec {
		commitConfig(*service, options, flags)
		restartService(*service)
	} else if *write {
		commitConfig(*service, options, flags)
	} else if *exec {
		if *service == "zookeeper" {
			log.Println("running zookeeper without writing it's config might not be what you want")
		}
		runInForeground(*service, options, flags)
	}
}

func txtRecords(service string, hostname string) map[string][]string {

	records := map[string][]string{}
	mutex := &sync.Mutex{}
	wg := sync.WaitGroup{}

	hostParts := strings.Split(hostname, ".")
	for i := range hostParts {
		domain := strings.Join(hostParts[i:], ".")

		for y := range nsprio[service] {
			dnsname := prefix + nsprio[service][y] + suffix + domain

			wg.Add(1)
			go func() {
				txt, err := net.LookupTXT(dnsname)
				if err != nil {
					dprint(fmt.Sprintf("%s", err))
				} else {
					dprint(fmt.Sprintf("lookup %s: found", dnsname))
					mutex.Lock()
					records[dnsname] = txt
					mutex.Unlock()
				}
				wg.Done()
			}()
		}
	}

	wg.Wait()

	return records
}

func findConfig(service string, hostname string) (map[string]string, []string) {
	options := map[string]string{}
	flags := map[string]bool{}

	records := txtRecords(service, hostname)

	// traverse through the hostname
	hostParts := strings.Split(hostname, ".")
	for i := range hostParts {
		domain := strings.Join(hostParts[i:], ".")

		// traverse through the services
		for y := range nsprio[service] {
			dnsname := prefix + nsprio[service][y] + suffix + domain

			txts := records[dnsname]

			// iterate all returned txt strings
			for t := range txts {

				s := strings.SplitN(txts[t], "=", 2)

				if len(s) == 1 {
					dprint(fmt.Sprintf("%s: enabling %s", dnsname, s[0]))
					flags[s[0]] = true
				} else if len(s) == 2 {
					if cv, ok := options[s[0]]; ok {
						dprint(fmt.Sprintf("option %s is already defined as %s, not overwriting with %s", s[0], cv, s[1]))
					} else {
						// due to the way configuration currently works the 'zk' entry in the .mesos.
						// hierarchy (/etc/mesos/zk) is being handled in a special way.
						if s[0] == "zk" && nsprio[service][y] == ".mesos." {

							// if we are dealing with marathon, we should derive zk and master options
							// from mesos config, if they were not passed to marathon directly
							if service == "marathon" {

								if _, ok := options["master"]; !ok {
									dprint(fmt.Sprintf("%s: setting master => %s", dnsname, s[1]))
									options["master"] = s[1]
								}
								zk := strings.Join(strings.Split(s[1], "/")[:3], "/") + "/marathon"
								dprint(fmt.Sprintf("%s: deriving %s => %s from %s", dnsname, s[0], zk, s[1]))
								s[1] = zk
							}
						}

						dprint(fmt.Sprintf("%s: found %s => %s", dnsname, s[0], s[1]))
						options[s[0]] = s[1]
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
	dprint(fmt.Sprintf("running: service %s restart", service))
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
		log.Println("running: zkServer.sh start-foreground")
		err = exec.Command("zkServer.sh", "start-foreground").Run()
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

func writeConfigFile(outDir string, option string, data []byte) {
	outDir = fsprefix + outDir

	outFile := outDir + option
	dprint(fmt.Sprintf("writing %s", outFile))

	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalln(err)
	}

	file, err := ioutil.TempFile(outDir, ".mesospherednsconfig")
	if err != nil {
		log.Fatalln(err)
	}

	if _, err = file.Write(data); err != nil {
		log.Fatalln(err)
	}

	if err = file.Close(); err != nil {
		log.Fatalln(err)
	}

	if err = os.Chmod(file.Name(), 0644); err != nil {
		log.Fatalln(err)
	}

	if err = os.Rename(file.Name(), outFile); err != nil {
		log.Fatalln(err)
	}
}

func writeMesosphereConfig(outDir string, options map[string]string, flags []string) {

	for option := range options {
		log.Printf("option: %s=%s\n", option, options[option])
		writeConfigFile(outDir, option, []byte(options[option]+"\n"))
	}

	for _, flag := range flags {
		log.Printf("flag: %s\n", flag)
		writeConfigFile(outDir, "?"+flag, []byte(""))
	}
}

func writeZookeeperConfig(myidDir string, zoocfgDir string, options map[string]string) {

	zoocfg := ""
	okeys := make([]string, len(options))
	i := 0
	for k, _ := range options {
		okeys[i] = k
		i++
	}
	sort.Strings(okeys)

	for _, option := range okeys {
		log.Printf("option: %s=%s\n", option, options[option])
		if option == "myid" {
			writeConfigFile(myidDir, option, []byte(options[option]+"\n"))
		} else {
			zoocfg += option + "=" + options[option] + "\n"
		}
	}

	if len(zoocfg) > 0 {
		writeConfigFile(zoocfgDir, "zoo.cfg", []byte(zoocfg))
	}
}
