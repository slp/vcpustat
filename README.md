## vcpustat

vcpustat is a simple utility to monitor the amount of Host CPU time each vCPU present in the system is using during a certain interval of real time (5 seconds by default), using Cgroups (Control Groups) accounting facilities.

## Usage

```
Usage of ./vcpustat:
  -f string
    	log to this file instead of stdout
  -i int
    	interval in seconds (default 5)
  -l int
    	only print CPU usages equal or lower than printlevel (default -1)
  -m int
    	put a (WARN) tag on CPU usages equal or lower than marklevel (default -1)
  -p int
    	trigger a Host panic if CPU usage is equal or lower than paniclevel (default -1)
```

## Triggering a panic

Even if it seems like an extreme measure, sometimes triggering a panic on the Host may be the only way for gathering scheduling and KVM data required to determine why a vCPU has received any CPU time from the Host.

This can be enabled with the "-p" argument, which can also be combined with other options.


## Installing vcpustat as a systemd service

This repository includes the file *vcpustat.service*, which can be used to run vcpustat as a systemd service. To do that, you can do something like this:

1. Edit vcpustat.service, setting the desired arguments for vcpustat on the *ExecStart* option
2. Copy vcpustat.service to /lib/systemd/system
3. Enable the service: ```systemctl enable vcpustat```
4. Start the service: ```systemctl start vcpustat```

## Examples

### Default behavior

When run without any argument, vcpustat will print all vCPU stats in a 5 seconds interval.

```
[slopezpa@kthompson vcpustat]$ ./vcpustat
2017-10-20 04:37:31.868849013 -0400 EDT
machine-qemu\x2drhel67\x2dtest.scope:
   vcpu0:                 527704
   vcpu1:                 485923
   vcpu2:                2641980
   vcpu3:                 683237
   vcpu4:                1019294
   vcpu5:                 506448
   vcpu6:                 377652
   vcpu7:                 290055
   vcpu8:                 219022
   vcpu9:                 333509
  vcpu10:                 587493
  vcpu11:                 488855
```

### Redirecting the output to a file.

The "-f" argument can be used to redirect the output to a file. Useful when ran as a daemon.

**WARNING** vcpustat will *not* rotate log files on its own, so you have to use another service for that, or rely on systemd logging.

### Highlighting low values

The "-m" argument can be used to highlight low values, so they can be easily found in the logfile.

```
[slopezpa@kthompson vcpustat]$ ./vcpustat -m 100000
2017-10-20 04:37:36.869667056 -0400 EDT
machine-qemu\x2drhel67\x2dtest.scope:
   vcpu0:                 379789
   vcpu1:                1190215
   vcpu2:                2740496
   vcpu3:                 376257
   vcpu4:                 377642
   vcpu5:                 459180
   vcpu6:                 363981
   vcpu7:                 366587
   vcpu8:                 413202
   vcpu9:                 492092
  vcpu10:                  96448 (WARN)
  vcpu11:                 320035
```

