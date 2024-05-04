# Prometheus Libvirt Exporter

Table of content:

- [Prometheus Libvirt Exporter](#prometheus-libvirt-exporter)
  - [0. Introduction](#0-introduction)
  - [1. Historical and Improvement](#1-historical-and-improvement)
  - [2. Installation and Usage](#2-installation-and-usage)
    - [2.1. Binary](#21-binary)
    - [2.2. Docker](#22-docker)
  - [3. Sample metrics](#3-sample-metrics)
  - [Libvirt/qemu version notice](#libvirtqemu-version-notice)

## 0. Introduction

This repository provides code for a Prometheus metrics exporter
for [libvirt](https://libvirt.org/). This exporter connects to any
libvirt daemon and exports per-domain metrics related to CPU, memory,
disk and network usage. By default, this exporter listens on TCP port 9177.

This exporter makes use of
[libvirt-go](https://gitlab.com/libvirt/libvirt-go-module), the official Go
bindings for libvirt. This exporter make use of the
`GetAllDomainStats()`.

## 1. Historical and Improvement

Project forked from <https://github.com/Tinkoff/libvirt-exporter> as it was archived and rewritten.

**Q**: Why don't just use <https://github.com/inovex/prometheus-libvirt-exporter>? It is active and maintained.

**A**: There are two main reasons:

- The [inovex/prometheus-libvirt-exporter](https://github.com/inovex/prometheus-libvirt-exporter) is not _compatible_ with old exporters, its metrics are different. I would like stick to old exporters in order to avoid modifying existing monitoring/alerting/presenting logic. And I know [I'm not the only one](https://github.com/inovex/prometheus-libvirt-exporter/issues/22).
- My system uses the Libvirt 6.0.0 and it doesn't support to [expose CPU steal metric](https://github.com/Tinkoff/libvirt-exporter?tab=readme-ov-file#libvirtqemu-version-notice). Therefore, the [inovex/prometheus-libvirt-exporter](https://github.com/inovex/prometheus-libvirt-exporter) doesn't solve my problem.

**Q**: So what's new?

**A**:

- ~~Some of the above might be exposed only with: `libvirt >= v7.2.0`: libvirt_domain_vcpu_delay_seconds_total~~ This libvirt-exporter calculates the CPU steal itself if the Libvirt doesn't expose the metrics.
- Up-to-date dependencies, also add logging and new Prometheus exporter web UI.

## 2. Installation and Usage

### 2.1. Binary

- You can use Golang install command to install libvirt-exporter:

```shell
$ go install github.com/ntk148v/libvirt-exporter@latest
# Run the binary
$ ~/go/bin/libvirt-exporter
```

- Or you can download the binary directly from [release page](https://github.com/ntk148v/libvirt-exporter/releases).
- Usage:

```shell
usage: libvirt-exporter [<flags>]


Flags:
  -h, --[no-]help                Show context-sensitive help (also try --help-long and --help-man).
      --path.procfs="/proc"      procfs mountpoint.
      --libvirt.uri="qemu:///system"
                                 Libvirt URI to extract metrics, available value: qemu:///system (default), qemu:///session, xen:///system and test:///default
      --web.telemetry-path="/metrics"
                                 Path under which to expose metrics
      --[no-]web.systemd-socket  Use systemd socket activation listeners instead of port listeners (Linux only).
      --web.listen-address=:9177 ...
                                 Addresses on which to expose metrics and web interface. Repeatable for multiple addresses.
      --web.config.file=""       Path to configuration file that can enable TLS or authentication. See: https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md
      --log.level=info           Only log messages with the given severity or above. One of: [debug, info, warn, error]
      --log.format=logfmt        Output format of log messages. One of: [logfmt, json]
      --[no-]version             Show application version.
```

### 2.2. Docker

The `libvirt-exporter` is designed to monitor the libvirt system by using Libvirt URI `/var/run/libvirt` and `/proc` (if Libvirt version < 7.2.0). Deploying in containers requires extra work to make it work properly.

If you start container for host monitoring, specify `path.procfs` argument. This argument must match path in bind-mount of host procfs (`/proc`). The `libvirt-exporter` will use `path.procfs` as prefix to access host filesystem. Another bind mount `/var/run/libvirt` is also required.

For Docker compose, use the [sample compose file](./docker-compose.yml):

```yaml
services:
  prometheus_libvirt_exporter:
    container_name: prometheus-libvirt-exporter
    image: kiennt26/prometheus-libvirt-exporter
    command:
      - "--path.procfs=/host/proc"
    network_mode: host
    pid: host
    restart: unless-stopped
    volumes:
      - "/:/host:ro,rslave"
      - "/var/run/libvirt:/var/run/libvirt"
```

## 3. Sample metrics

The following metrics/labels are being exported:

```
libvirt_domain_block_meta{bus="scsi",cache="none",discard="unmap",disk_type="network",domain="instance-00000337",driver_type="raw",serial="5f1a922c-e4b5-4020-9308-d70fd8219ac8",source_file="somepool/volume-5f1a922c-e4b5-4020-9308-d70fd8219ac8",target_device="sda"} 1
libvirt_domain_block_stats_allocation{domain="instance-00000337",target_device="sda"} 2.1474816e+10
libvirt_domain_block_stats_capacity_bytes{domain="instance-00000337",target_device="sda"} 2.147483648e+10
libvirt_domain_block_stats_flush_requests_total{domain="instance-00000337",target_device="sda"} 5.153142e+06
libvirt_domain_block_stats_flush_time_seconds_total{domain="instance-00000337",target_device="sda"} 473.56850521
libvirt_domain_block_stats_limit_burst_length_read_requests_seconds{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_length_total_requests_seconds{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_length_write_requests_seconds{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_read_bytes{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_read_bytes_length_seconds{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_read_requests{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_total_bytes{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_total_bytes_length_seconds{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_total_requests{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_write_bytes{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_write_bytes_length_seconds{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_burst_write_requests{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_read_bytes{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_read_requests{domain="instance-00000337",target_device="sda"} 640
libvirt_domain_block_stats_limit_total_bytes{domain="instance-00000337",target_device="sda"} 1.572864e+08
libvirt_domain_block_stats_limit_total_requests{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_write_bytes{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_limit_write_requests{domain="instance-00000337",target_device="sda"} 320
libvirt_domain_block_stats_physicalsize_bytes{domain="instance-00000337",target_device="sda"} 2.147483648e+10
libvirt_domain_block_stats_read_bytes_total{domain="instance-00000337",target_device="sda"} 1.7704034304e+11
libvirt_domain_block_stats_read_requests_total{domain="instance-00000337",target_device="sda"} 1.9613982e+07
libvirt_domain_block_stats_read_time_seconds_total{domain="instance-00000337",target_device="sda"} 161803.085086353
libvirt_domain_block_stats_size_iops_bytes{domain="instance-00000337",target_device="sda"} 0
libvirt_domain_block_stats_write_bytes_total{domain="instance-00000337",target_device="sda"} 9.2141217792e+11
libvirt_domain_block_stats_write_requests_total{domain="instance-00000337",target_device="sda"} 2.8434899e+07
libvirt_domain_block_stats_write_time_seconds_total{domain="instance-00000337",target_device="sda"} 530522.437009019

libvirt_pool_info_allocation_bytes{pool="default"} 5.4276182016e+10
libvirt_pool_info_available_bytes{pool="default"} 5.1278647296e+10
libvirt_pool_info_capacity_bytes{pool="default"} 1.05554829312e+11

libvirt_domain_info_cpu_time_seconds_total{domain="instance-00000337"} 949422.12
libvirt_domain_info_maximum_memory_bytes{domain="instance-00000337"} 8.589934592e+09
libvirt_domain_info_memory_usage_bytes{domain="instance-00000337"} 8.589934592e+09
libvirt_domain_info_meta{domain="instance-00000337",flavor="someflavor-8192",instance_name="name.of.instance.com",project_name="instance.com",project_uuid="3051f6f46d394ab98f55a0670ae5c70b",root_type="image",root_uuid="155e5ab9-d28c-48f2-bd8d-f193d0a6128a",user_name="master_admin",user_uuid="240270fa2a3e4fd3baa6d6e776669b19",uuid="1bac351f-242e-4d53-8cf3-fd91b061069c"} 1
libvirt_domain_info_virtual_cpus{domain="instance-00000337"} 2
libvirt_domain_info_vstate{domain="instance-00000337"} 1

libvirt_domain_interface_meta{domain="instance-00000337",source_bridge="br-int",target_device="tapa7e2fe95-a7",virtual_interface="a7e2fe95-a7cf-4bec-8180-d835cf342d72"} 1
libvirt_domain_interface_stats_receive_bytes_total{domain="instance-00000337",target_device="tapa7e2fe95-a7"} 7.9182281e+09
libvirt_domain_interface_stats_receive_drops_total{domain="instance-00000337",target_device="tapa7e2fe95-a7"} 0
libvirt_domain_interface_stats_receive_errors_total{domain="instance-00000337",target_device="tapa7e2fe95-a7"} 0
libvirt_domain_interface_stats_receive_packets_total{domain="instance-00000337",target_device="tapa7e2fe95-a7"} 4.378193e+06
libvirt_domain_interface_stats_transmit_bytes_total{domain="instance-00000337",target_device="tapa7e2fe95-a7"} 1.819996331e+09
libvirt_domain_interface_stats_transmit_drops_total{domain="instance-00000337",target_device="tapa7e2fe95-a7"} 0
libvirt_domain_interface_stats_transmit_errors_total{domain="instance-00000337",target_device="tapa7e2fe95-a7"} 0
libvirt_domain_interface_stats_transmit_packets_total{domain="instance-00000337",target_device="tapa7e2fe95-a7"} 2.275386e+06

libvirt_domain_memory_stats_actual_balloon_bytes{domain="instance-00000337"} 8.589934592e+09
libvirt_domain_memory_stats_available_bytes{domain="instance-00000337"} 8.363945984e+09
libvirt_domain_memory_stats_disk_cache_bytes{domain="instance-00000337"} 0
libvirt_domain_memory_stats_major_fault_total{domain="instance-00000337"} 3.34448e+06
libvirt_domain_memory_stats_minor_fault_total{domain="instance-00000337"} 5.6630255354e+10
libvirt_domain_memory_stats_rss_bytes{domain="instance-00000337"} 8.7020544e+09
libvirt_domain_memory_stats_unused_bytes{domain="instance-00000337"} 7.72722688e+08
libvirt_domain_memory_stats_usable_bytes{domain="instance-00000337"} 2.27098624e+09
libvirt_domain_memory_stats_used_percent{domain="instance-00000337"} 72.84790881786736

libvirt_domain_vcpu_cpu{domain="instance-00000337",vcpu="0"} 7
libvirt_domain_vcpu_delay_seconds_total{domain="instance-00000337",vcpu="0"} 880.985415109
libvirt_domain_vcpu_state{domain="instance-00000337",vcpu="0"} 1
libvirt_domain_vcpu_time_seconds_total{domain="instance-00000337",vcpu="0"} 315190.41
libvirt_domain_vcpu_wait_seconds_total{domain="instance-00000337",vcpu="0"} 0

libvirt_up 1
```

## Libvirt/qemu version notice

Some of the above might be exposed only with:

`libvirt >= v7.2.0`:
libvirt_domain_vcpu_delay_seconds_total
