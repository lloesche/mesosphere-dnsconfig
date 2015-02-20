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
```
# ./mesosphere-dnsconfig
  -dry-run=false: dry run: do not write configs, just print them
  -hostname="": hostname to use, os hostname is used by default
  -service="": service to configure: mesos, mesos-master, mesos-slave, marathon or zookeeper
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
- zookeeper: /var/lib/zookeeper/myid and /etc/zookeeper/conf/zoo.cfg

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


***Extended Example***
```
$ host -t txt config.marathon._mesosphere.hw.ca1.mesosphere.com
config.marathon._mesosphere.hw.ca1.mesosphere.com descriptive text "checkpoint"
config.marathon._mesosphere.hw.ca1.mesosphere.com descriptive text "event_subscriber=http_callback"
config.marathon._mesosphere.hw.ca1.mesosphere.com descriptive text "framework_name=marathon"
config.marathon._mesosphere.hw.ca1.mesosphere.com descriptive text "task_launch_timeout=300000"

$ host -t txt config.mesos._mesosphere.hw.ca1.mesosphere.com
config.mesos._mesosphere.hw.ca1.mesosphere.com descriptive text "zk=zk://172.16.0.12:2181,172.16.0.13:2181,172.16.0.14:2181/mesos"

$ host -t txt config.mesos-master._mesosphere.hw.ca1.mesosphere.com
config.mesos-master._mesosphere.hw.ca1.mesosphere.com descriptive text "cluster=OVH1"
config.mesos-master._mesosphere.hw.ca1.mesosphere.com descriptive text "quorum=2"
config.mesos-master._mesosphere.hw.ca1.mesosphere.com descriptive text "work_dir=/hdd/mesos/master"

$ host -t txt config.mesos-slave._mesosphere.hw.ca1.mesosphere.com
config.mesos-slave._mesosphere.hw.ca1.mesosphere.com descriptive text "containerizers=docker,mesos"
config.mesos-slave._mesosphere.hw.ca1.mesosphere.com descriptive text "executor_registration_timeout=5mins"
config.mesos-slave._mesosphere.hw.ca1.mesosphere.com descriptive text "isolation=cgroups/cpu,cgroups/mem"
config.mesos-slave._mesosphere.hw.ca1.mesosphere.com descriptive text "work_dir=/hdd/mesos/slave"

$ host -t txt config.mesos-slave._mesosphere.srv2.hw.ca1.mesosphere.com
config.mesos-slave._mesosphere.srv2.hw.ca1.mesosphere.com descriptive text "hostname=srv2.hw.ca1.mesosphere.com"

$ host -t txt config.mesos-master._mesosphere.srv2.hw.ca1.mesosphere.com
config.mesos-master._mesosphere.srv2.hw.ca1.mesosphere.com descriptive text "hostname=srv2.hw.ca1.mesosphere.com"

$ host -t txt config.zookeeper._mesosphere.srv2.hw.ca1.mesosphere.com
config.zookeeper._mesosphere.srv2.hw.ca1.mesosphere.com descriptive text "myid=1"

$ host -t txt config.zookeeper._mesosphere.hw.ca1.mesosphere.com
config.zookeeper._mesosphere.hw.ca1.mesosphere.com descriptive text "clientPort=2181"
config.zookeeper._mesosphere.hw.ca1.mesosphere.com descriptive text "dataDir=/var/lib/zookeeper"
config.zookeeper._mesosphere.hw.ca1.mesosphere.com descriptive text "initLimit=10"
config.zookeeper._mesosphere.hw.ca1.mesosphere.com descriptive text "maxClientCnxns=60"
config.zookeeper._mesosphere.hw.ca1.mesosphere.com descriptive text "server.1=172.16.0.12:2888:3888"
config.zookeeper._mesosphere.hw.ca1.mesosphere.com descriptive text "server.2=172.16.0.13:2888:3888"
config.zookeeper._mesosphere.hw.ca1.mesosphere.com descriptive text "server.3=172.16.0.14:2888:3888"
config.zookeeper._mesosphere.hw.ca1.mesosphere.com descriptive text "syncLimit=5"
config.zookeeper._mesosphere.hw.ca1.mesosphere.com descriptive text "tickTime=2000"
```
