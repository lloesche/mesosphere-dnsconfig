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


Paths
-----
Right now the following paths are hard coded:
- mesos: /etc/mesos/
- mesos-master: /etc/mesos-master/
- mesos-slave: /etc/mesos-slave/
- marathon: /etc/marathon/conf/
- zookeeper: /var/lib/zookeeper/myid and /etc/zookeeper/zoo.cfg

For Mesosphere services configuration files are being created in the Mesosphere filename=key, file_content=value style. For Zookeeper the key myid will be written to /var/lib/zookeeper/myid, everything else is written to /etc/zookeeper/zoo.cfg.

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
