# mesosphere-dnsconfig
Generate Mesosphere configuration files from DNS data

Set Up
------
* Install Go.
* Build source
```bash
go build mesosphere-dnsconfig.go
```

Usage
-----
```bash
mesosphere-dnsconfig <service> [hostname]
```

service can be one of
- mesos
- mesos-master
- mesos-slave
- marathon
- zookeeper

If hostname is not specified the name that's being returned by `hostname` will be used.

DNS Setup
---------
mesosphere-dnsconfig starts by querying the nameserver for the following TXT record:

`config.<service>._mesosphere.<hostname-parts>`

It then continues down the name hirarchy.

***Example***
- Hostname: srv2.hw.ca1.mesosphere.com
- Service: marathon

TXT record lookups:
```
config.marathon._mesosphere.srv2.hw.ca1.mesosphere.com
     config.marathon._mesosphere.hw.ca1.mesosphere.com
        config.marathon._mesosphere.ca1.mesosphere.com
            config.marathon._mesosphere.mesosphere.com
                       config.marathon._mesosphere.com
```

The contents of the TXT records are either 'key=value' pairs for options or just 'key' for flags.
E.g.
```
config.marathon._mesosphere.mesosphere.com  IN  TXT "task_launch_timeout=300000"
                                            IN  TXT "checkpoint"
```
becomes the arguments `--task_launch_timeout=300000 --checkpoint`.
