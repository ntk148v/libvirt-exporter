// Copyright 2024 Kien Nguyen Tuan
// Copyright 2021 Aleksei Zakharov, https://alexzzz.ru/
// Copyright 2017 Kumina, https://kumina.nl/
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Project forked from https://github.com/Tinkoff/libvirt-exporter

package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"github.com/prometheus/procfs"
	"libvirt.org/go/libvirt"

	"github.com/ntk148v/libvirt-exporter/pkg/libvirtSchema"
	"github.com/ntk148v/libvirt-exporter/pkg/utils"
)

var (
	libvirtUpDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "", "up"),
		"Whether scraping libvirt's metrics was successful.",
		nil,
		nil)
	libvirtPoolInfoCapacity = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "pool_info", "capacity_bytes"),
		"Pool capacity, in bytes",
		[]string{"pool"},
		nil)
	libvirtPoolInfoAllocation = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "pool_info", "allocation_bytes"),
		"Pool allocation, in bytes",
		[]string{"pool"},
		nil)
	libvirtPoolInfoAvailable = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "pool_info", "available_bytes"),
		"Pool available, in bytes",
		[]string{"pool"},
		nil)
	libvirtVersionsInfoDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "", "versions_info"),
		"Versions of virtualization components",
		[]string{"hypervisor_running", "libvirtd_running", "libvirt_library"},
		nil)
	libvirtDomainInfoMetaDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_info", "meta"),
		"Domain metadata",
		[]string{"domain", "uuid", "instance_name", "flavor", "user_name", "user_uuid", "project_name", "project_uuid", "root_type", "root_uuid"},
		nil)
	libvirtDomainInfoMaxMemBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_info", "maximum_memory_bytes"),
		"Maximum allowed memory of the domain, in bytes.",
		[]string{"domain"},
		nil)
	libvirtDomainInfoMemoryUsageBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_info", "memory_usage_bytes"),
		"Memory usage of the domain, in bytes.",
		[]string{"domain"},
		nil)
	libvirtDomainInfoNrVirtCPUDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_info", "virtual_cpus"),
		"Number of virtual CPUs for the domain.",
		[]string{"domain"},
		nil)
	libvirtDomainInfoCPUTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_info", "cpu_time_seconds_total"),
		"Amount of CPU time used by the domain, in seconds.",
		[]string{"domain"},
		nil)
	libvirtDomainInfoVirDomainState = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_info", "vstate"),
		"Virtual domain state. 0: no state, 1: the domain is running, 2: the domain is blocked on resource,"+
			" 3: the domain is paused by user, 4: the domain is being shut down, 5: the domain is shut off,"+
			"6: the domain is crashed, 7: the domain is suspended by guest power management",
		[]string{"domain"},
		nil)

	libvirtDomainVcpuTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_vcpu", "time_seconds_total"),
		"Amount of CPU time used by the domain's VCPU, in seconds.",
		[]string{"domain", "vcpu"},
		nil)
	libvirtDomainVcpuDelayDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_vcpu", "delay_seconds_total"),
		"Amount of CPU time used by the domain's VCPU, in seconds. "+
			"Vcpu's delay metric. Time the vcpu thread was enqueued by the "+
			"host scheduler, but was waiting in the queue instead of running. "+
			"Exposed to the VM as a steal time.",
		[]string{"domain", "vcpu"},
		nil)
	libvirtDomainVcpuStateDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_vcpu", "state"),
		"VCPU state. 0: offline, 1: running, 2: blocked",
		[]string{"domain", "vcpu"},
		nil)
	libvirtDomainVcpuCPUDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_vcpu", "cpu"),
		"Real CPU number, or one of the values from virVcpuHostCpuState",
		[]string{"domain", "vcpu"},
		nil)
	libvirtDomainVcpuWaitDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_vcpu", "wait_seconds_total"),
		"Vcpu's wait_sum metric. CONFIG_SCHEDSTATS has to be enabled",
		[]string{"domain", "vcpu"},
		nil)

	libvirtDomainMetaBlockDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block", "meta"),
		"Block device metadata info. Device name, source file, serial.",
		[]string{"domain", "target_device", "source_file", "serial", "bus", "disk_type", "driver_type", "cache", "discard"},
		nil)
	libvirtDomainBlockRdBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "read_bytes_total"),
		"Number of bytes read from a block device, in bytes.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockRdReqDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "read_requests_total"),
		"Number of read requests from a block device.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockRdTotalTimeSecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "read_time_seconds_total"),
		"Total time spent on reads from a block device, in seconds.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockWrBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "write_bytes_total"),
		"Number of bytes written to a block device, in bytes.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockWrReqDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "write_requests_total"),
		"Number of write requests to a block device.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockWrTotalTimesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "write_time_seconds_total"),
		"Total time spent on writes on a block device, in seconds",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockFlushReqDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "flush_requests_total"),
		"Total flush requests from a block device.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockFlushTotalTimeSecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "flush_time_seconds_total"),
		"Total time in seconds spent on cache flushing to a block device",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockAllocationDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "allocation"),
		"Offset of the highest written sector on a block device.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockCapacityBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "capacity_bytes"),
		"Logical size in bytes of the block device	backing image.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockPhysicalSizeBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "physicalsize_bytes"),
		"Physical size in bytes of the container of the backing image.",
		[]string{"domain", "target_device"},
		nil)

	// Block IO tune parameters
	// Limits
	libvirtDomainBlockTotalBytesSecDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_total_bytes"),
		"Total throughput limit in bytes per second",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockWriteBytesSecDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_write_bytes"),
		"Write throughput limit in bytes per second",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockReadBytesSecDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_read_bytes"),
		"Read throughput limit in bytes per second",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockTotalIopsSecDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_total_requests"),
		"Total requests per second limit",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockWriteIopsSecDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_write_requests"),
		"Write requests per second limit",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockReadIopsSecDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_read_requests"),
		"Read requests per second limit",
		[]string{"domain", "target_device"},
		nil)
	// Burst limits
	libvirtDomainBlockTotalBytesSecMaxDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_total_bytes"),
		"Total throughput burst limit in bytes per second",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockWriteBytesSecMaxDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_write_bytes"),
		"Write throughput burst limit in bytes per second",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockReadBytesSecMaxDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_read_bytes"),
		"Read throughput burst limit in bytes per second",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockTotalIopsSecMaxDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_total_requests"),
		"Total requests per second burst limit",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockWriteIopsSecMaxDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_write_requests"),
		"Write requests per second burst limit",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockReadIopsSecMaxDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_read_requests"),
		"Read requests per second burst limit",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockTotalBytesSecMaxLengthDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_total_bytes_length_seconds"),
		"Total throughput burst time in seconds",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockWriteBytesSecMaxLengthDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_write_bytes_length_seconds"),
		"Write throughput burst time in seconds",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockReadBytesSecMaxLengthDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_read_bytes_length_seconds"),
		"Read throughput burst time in seconds",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockTotalIopsSecMaxLengthDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_length_total_requests_seconds"),
		"Total requests per second burst time in seconds",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockWriteIopsSecMaxLengthDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_length_write_requests_seconds"),
		"Write requests per second burst time in seconds",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockReadIopsSecMaxLengthDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "limit_burst_length_read_requests_seconds"),
		"Read requests per second burst time in seconds",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainBlockSizeIopsSecDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_block_stats", "size_iops_bytes"),
		"The size of IO operations per second permitted through a block device",
		[]string{"domain", "target_device"},
		nil)

	libvirtDomainMetaInterfacesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_interface", "meta"),
		"Interfaces metadata. Source bridge, target device, interface uuid",
		[]string{"domain", "source_bridge", "target_device", "virtual_interface"},
		nil)
	libvirtDomainInterfaceRxBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_interface_stats", "receive_bytes_total"),
		"Number of bytes received on a network interface, in bytes.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainInterfaceRxPacketsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_interface_stats", "receive_packets_total"),
		"Number of packets received on a network interface.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainInterfaceRxErrsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_interface_stats", "receive_errors_total"),
		"Number of packet receive errors on a network interface.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainInterfaceRxDropDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_interface_stats", "receive_drops_total"),
		"Number of packet receive drops on a network interface.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainInterfaceTxBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_interface_stats", "transmit_bytes_total"),
		"Number of bytes transmitted on a network interface, in bytes.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainInterfaceTxPacketsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_interface_stats", "transmit_packets_total"),
		"Number of packets transmitted on a network interface.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainInterfaceTxErrsDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_interface_stats", "transmit_errors_total"),
		"Number of packet transmit errors on a network interface.",
		[]string{"domain", "target_device"},
		nil)
	libvirtDomainInterfaceTxDropDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_interface_stats", "transmit_drops_total"),
		"Number of packet transmit drops on a network interface.",
		[]string{"domain", "target_device"},
		nil)

	libvirtDomainMemoryStatMajorFaultTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_memory_stats", "major_fault_total"),
		"Page faults occur when a process makes a valid access to virtual memory that is not available. "+
			"When servicing the page fault, if disk IO is required, it is considered a major fault.",
		[]string{"domain"},
		nil)
	libvirtDomainMemoryStatMinorFaultTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_memory_stats", "minor_fault_total"),
		"Page faults occur when a process makes a valid access to virtual memory that is not available. "+
			"When servicing the page not fault, if disk IO is required, it is considered a minor fault.",
		[]string{"domain"},
		nil)
	libvirtDomainMemoryStatUnusedBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_memory_stats", "unused_bytes"),
		"The amount of memory left completely unused by the system. Memory that is available but used for "+
			"reclaimable caches should NOT be reported as free. This value is expressed in bytes.",
		[]string{"domain"},
		nil)
	libvirtDomainMemoryStatAvailableBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_memory_stats", "available_bytes"),
		"The total amount of usable memory as seen by the domain. This value may be less than the amount of "+
			"memory assigned to the domain if a balloon driver is in use or if the guest OS does not initialize all "+
			"assigned pages. This value is expressed in bytes.",
		[]string{"domain"},
		nil)
	libvirtDomainMemoryStatActualBaloonBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_memory_stats", "actual_balloon_bytes"),
		"Current balloon value (in bytes).",
		[]string{"domain"},
		nil)
	libvirtDomainMemoryStatRssBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_memory_stats", "rss_bytes"),
		"Resident Set Size of the process running the domain. This value is in bytes",
		[]string{"domain"},
		nil)
	libvirtDomainMemoryStatUsableBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_memory_stats", "usable_bytes"),
		"How much the balloon can be inflated without pushing the guest system to swap, corresponds "+
			"to 'Available' in /proc/meminfo",
		[]string{"domain"},
		nil)
	libvirtDomainMemoryStatDiskCachesBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_memory_stats", "disk_cache_bytes"),
		"The amount of memory, that can be quickly reclaimed without additional I/O (in bytes)."+
			"Typically these pages are used for caching files from disk.",
		[]string{"domain"},
		nil)
	libvirtDomainMemoryStatUsedPercentDesc = prometheus.NewDesc(
		prometheus.BuildFQName("libvirt", "domain_memory_stats", "used_percent"),
		"The amount of memory in percent, that used by domain.",
		[]string{"domain"},
		nil)

	errorsMap map[string]struct{}

	// The list of host processes
	processes []int

	// The path of the proc filesystem.
	procFSPath = kingpin.Flag("path.procfs", "procfs mountpoint.").Default(procfs.DefaultMountPoint).String()
)

// WriteErrorOnce writes message to stdout only once
// for the error
// "err" - an error message
// "name" - name of an error, to count it
func WriteErrorOnce(err string, name string, logger log.Logger) {
	if _, ok := errorsMap[name]; !ok {
		_ = level.Error(logger).Log("err", err)
		errorsMap[name] = struct{}{}
	}
}

// GetDomainPid returns the VM's Pid by iterating over process list
func GetDomainPid(domainName string) (pid int) {
	// lookup PID
	for _, process := range processes {
		cmdline := utils.GetCmdLine(*procFSPath, process)
		if cmdline != "" && strings.Contains(cmdline, domainName) {
			// fmt.Printf("Found PID %d for instance %s (cmdline: %s)", process, name, cmdline)
			pid = process
			break
		}
	}

	return
}

// GetDomainVcpuPids returns the list of vcpu pid.
// It runs the following command:
//
// virsh -c qemu:///system qemu-monitor-command --hmp <domain-name> info cpus
//   - CPU #0: thread_id=151260
//     CPU #1: thread_id=151261
//
// Then get the thread ids.
func GetDomainVcpuPids(domain *libvirt.Domain) (vCPUPids []int, err error) {
	// NOTE(kiennt): For the libvirt version < v7.2.0, we have to self-calculate CPU steal
	// Get the thread ids or VCPU's pid.
	vCPUThreads, err := domain.QemuMonitorCommand("info cpus", libvirt.DOMAIN_QEMU_MONITOR_COMMAND_HMP)
	if err != nil {
		return vCPUPids, err
	}

	regThreadID := regexp.MustCompile(`thread_id=([0-9]*)\s`)
	threadIDsRaw := regThreadID.FindAllStringSubmatch(vCPUThreads, -1)
	vCPUPids = make([]int, len(threadIDsRaw))
	for i, thread := range threadIDsRaw {
		threadID, _ := strconv.Atoi(thread[1])
		vCPUPids[i] = threadID
	}

	return
}

// CollectDomain extracts Prometheus metrics from a libvirt domain.
func CollectDomain(ch chan<- prometheus.Metric, stat libvirt.DomainStats, logger log.Logger) error {
	domainName, err := stat.Domain.GetName()
	if err != nil {
		return err
	}

	// Get Domain PID and its Vcpu Pids
	domainPid := GetDomainPid(domainName)
	domainVcpuPids, err := GetDomainVcpuPids(stat.Domain)
	if err != nil {
		lverr, ok := err.(libvirt.Error)
		if !ok || lverr.Code != libvirt.ERR_OPERATION_INVALID {
			return err
		}
	}

	domainUUID, err := stat.Domain.GetUUIDString()
	if err != nil {
		return err
	}

	// Decode XML description of domain to get block device names, etc.
	xmlDesc, err := stat.Domain.GetXMLDesc(0)
	if err != nil {
		return err
	}
	var desc libvirtSchema.Domain
	err = xml.Unmarshal([]byte(xmlDesc), &desc)
	if err != nil {
		return err
	}

	// Report domain info.
	info, err := stat.Domain.GetInfo()
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainInfoMetaDesc,
		prometheus.GaugeValue,
		float64(1),
		domainName,
		domainUUID,
		desc.Metadata.NovaInstance.NovaName,
		desc.Metadata.NovaInstance.NovaFlavor.FlavorName,
		desc.Metadata.NovaInstance.NovaOwner.NovaUser.UserName,
		desc.Metadata.NovaInstance.NovaOwner.NovaUser.UserUUID,
		desc.Metadata.NovaInstance.NovaOwner.NovaProject.ProjectName,
		desc.Metadata.NovaInstance.NovaOwner.NovaProject.ProjectUUID,
		desc.Metadata.NovaInstance.NovaRoot.RootType,
		desc.Metadata.NovaInstance.NovaRoot.RootUUID)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainInfoMaxMemBytesDesc,
		prometheus.GaugeValue,
		float64(info.MaxMem)*1024,
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainInfoMemoryUsageBytesDesc,
		prometheus.GaugeValue,
		float64(info.Memory)*1024,
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainInfoNrVirtCPUDesc,
		prometheus.GaugeValue,
		float64(info.NrVirtCpu),
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainInfoCPUTimeDesc,
		prometheus.CounterValue,
		float64(info.CpuTime)/1000/1000/1000, // From nsec to sec
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainInfoVirDomainState,
		prometheus.GaugeValue,
		float64(info.State),
		domainName)

	domainStatsVcpu, err := stat.Domain.GetVcpus()
	if err != nil {
		lverr, ok := err.(libvirt.Error)
		if !ok || lverr.Code != libvirt.ERR_OPERATION_INVALID {
			return err
		}
	} else {
		for _, vcpu := range domainStatsVcpu {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainVcpuStateDesc,
				prometheus.GaugeValue,
				float64(vcpu.State),
				domainName,
				strconv.FormatInt(int64(vcpu.Number), 10))

			ch <- prometheus.MustNewConstMetric(
				libvirtDomainVcpuTimeDesc,
				prometheus.CounterValue,
				float64(vcpu.CpuTime)/1000/1000/1000, // From nsec to sec
				domainName,
				strconv.FormatInt(int64(vcpu.Number), 10))

			ch <- prometheus.MustNewConstMetric(
				libvirtDomainVcpuCPUDesc,
				prometheus.GaugeValue,
				float64(vcpu.Cpu),
				domainName,
				strconv.FormatInt(int64(vcpu.Number), 10))
		}

		/* There's no Wait in GetVcpus()
		 * But there's no cpu number in libvirt.DomainStats
		 * Time and State are present in both structs
		 * So, let's take Wait here
		 */
		for cpuNum, vcpu := range stat.Vcpu {
			if vcpu.WaitSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainVcpuWaitDesc,
					prometheus.CounterValue,
					float64(vcpu.Wait)/1000/1000/1000,
					domainName,
					strconv.FormatInt(int64(cpuNum), 10))
			}
			if vcpu.DelaySet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainVcpuDelayDesc,
					prometheus.CounterValue,
					float64(vcpu.Delay)/1e9,
					domainName,
					strconv.FormatInt(int64(cpuNum), 10))
			} else {
				// stat.Vcpu == VcpuMaximum  [1]
				// But sometimes, the virtual machine doesn't use all maximum vcpu.
				// For example:
				// 		vcpu.current = 4
				//      vcpu.maximum = 48
				// [1] https://github.com/libvirt/libvirt-go/blob/master/connect.go#L3179
				if len(domainVcpuPids) <= cpuNum {
					continue
				}

				// If there are no vcpu delay measurement, we calculate it ourselves.
				vcpuPid := domainVcpuPids[cpuNum]
				procFSSchedStat, err := utils.GetProcPIDSchedStat(filepath.Join(*procFSPath, strconv.Itoa(domainPid), "task"), vcpuPid)
				if err != nil {
					_ = level.Error(logger).Log("err", "unable to collect vcpu delay metric", "msg", err)
					continue
				}
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainVcpuDelayDesc,
					prometheus.CounterValue,
					float64(procFSSchedStat.Runqueue)/1e9,
					domainName,
					strconv.FormatInt(int64(cpuNum), 10))
			}
		}
	}

	// Report block device statistics.
	for _, disk := range stat.Block {
		var DiskSource string
		var Device *libvirtSchema.Disk
		// Ugly hack to avoid getting metrics from cdrom block device
		// TODO: somehow check the disk 'device' field for 'cdrom' string
		if disk.Name == "hdc" || disk.Name == "hda" {
			continue
		}
		/*  "block.<num>.path" - string describing the source of block device <num>,
		    if it is a file or block device (omitted for network
		    sources and drives with no media inserted). For network device (i.e. rbd) take from xml. */
		for _, dev := range desc.Devices.Disks {
			if dev.Target.Device == disk.Name {
				if disk.PathSet {
					DiskSource = disk.Path

				} else {
					DiskSource = dev.Source.Name
				}
				Device = &dev
				break
			}
		}

		ch <- prometheus.MustNewConstMetric(
			libvirtDomainMetaBlockDesc,
			prometheus.GaugeValue,
			float64(1),
			domainName,
			disk.Name,
			DiskSource,
			Device.Serial,
			Device.Target.Bus,
			Device.DiskType,
			Device.Driver.Type,
			Device.Driver.Cache,
			Device.Driver.Discard,
		)

		// https://libvirt.org/html/libvirt-libvirt-domain.html#virConnectGetAllDomainStats
		if disk.RdBytesSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockRdBytesDesc,
				prometheus.CounterValue,
				float64(disk.RdBytes),
				domainName,
				disk.Name)
		}
		if disk.RdReqsSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockRdReqDesc,
				prometheus.CounterValue,
				float64(disk.RdReqs),
				domainName,
				disk.Name)
		}
		if disk.RdTimesSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockRdTotalTimeSecondsDesc,
				prometheus.CounterValue,
				float64(disk.RdTimes)/1e9,
				domainName,
				disk.Name)
		}
		if disk.WrBytesSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockWrBytesDesc,
				prometheus.CounterValue,
				float64(disk.WrBytes),
				domainName,
				disk.Name)
		}
		if disk.WrReqsSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockWrReqDesc,
				prometheus.CounterValue,
				float64(disk.WrReqs),
				domainName,
				disk.Name)
		}
		if disk.WrTimesSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockWrTotalTimesDesc,
				prometheus.CounterValue,
				float64(disk.WrTimes)/1e9,
				domainName,
				disk.Name)
		}
		if disk.FlReqsSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockFlushReqDesc,
				prometheus.CounterValue,
				float64(disk.FlReqs),
				domainName,
				disk.Name)
		}
		if disk.FlTimesSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockFlushTotalTimeSecondsDesc,
				prometheus.CounterValue,
				float64(disk.FlTimes)/1e9,
				domainName,
				disk.Name)
		}
		if disk.AllocationSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockAllocationDesc,
				prometheus.GaugeValue,
				float64(disk.Allocation),
				domainName,
				disk.Name)
		}
		if disk.CapacitySet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockCapacityBytesDesc,
				prometheus.GaugeValue,
				float64(disk.Capacity),
				domainName,
				disk.Name)
		}
		if disk.PhysicalSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainBlockPhysicalSizeBytesDesc,
				prometheus.GaugeValue,
				float64(disk.Physical),
				domainName,
				disk.Name)
		}

		blockIOTuneParams, err := stat.Domain.GetBlockIoTune(disk.Name, 0)
		if err != nil {
			lverr, ok := err.(libvirt.Error)
			if !ok {
				switch lverr.Code {
				case libvirt.ERR_OPERATION_INVALID:
					// This should be one-shot error
					_ = level.Error(logger).Log("err", "invalid operation GetBlockIoTune", "msg", err)
				case libvirt.ERR_OPERATION_UNSUPPORTED:
					WriteErrorOnce("Unsupported operation GetBlockIoTune: "+err.Error(), "blkiotune_unsupported", logger)
				default:
					return err
				}
			}
		} else {
			if blockIOTuneParams.TotalBytesSecSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockTotalBytesSecDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.TotalBytesSec),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.ReadBytesSecSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockReadBytesSecDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.ReadBytesSec),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.WriteBytesSecSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockWriteBytesSecDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.WriteBytesSec),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.TotalIopsSecSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockTotalIopsSecDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.TotalIopsSec),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.ReadIopsSecSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockReadIopsSecDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.ReadIopsSec),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.WriteIopsSecSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockWriteIopsSecDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.WriteIopsSec),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.TotalBytesSecMaxSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockTotalBytesSecMaxDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.TotalBytesSecMax),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.ReadBytesSecMaxSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockReadBytesSecMaxDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.ReadBytesSecMax),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.WriteBytesSecMaxSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockWriteBytesSecMaxDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.WriteBytesSecMax),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.TotalIopsSecMaxSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockTotalIopsSecMaxDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.TotalIopsSecMax),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.ReadIopsSecMaxSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockReadIopsSecMaxDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.ReadIopsSecMax),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.WriteIopsSecMaxSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockWriteIopsSecMaxDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.WriteIopsSecMax),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.TotalBytesSecMaxLengthSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockTotalBytesSecMaxLengthDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.TotalBytesSecMaxLength),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.ReadBytesSecMaxLengthSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockReadBytesSecMaxLengthDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.ReadBytesSecMaxLength),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.WriteBytesSecMaxLengthSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockWriteBytesSecMaxLengthDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.WriteBytesSecMaxLength),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.TotalIopsSecMaxLengthSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockTotalIopsSecMaxLengthDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.TotalIopsSecMaxLength),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.ReadIopsSecMaxLengthSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockReadIopsSecMaxLengthDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.ReadIopsSecMaxLength),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.WriteIopsSecMaxLengthSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockWriteIopsSecMaxLengthDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.WriteIopsSecMaxLength),
					domainName,
					disk.Name)
			}
			if blockIOTuneParams.SizeIopsSecSet {
				ch <- prometheus.MustNewConstMetric(
					libvirtDomainBlockSizeIopsSecDesc,
					prometheus.GaugeValue,
					float64(blockIOTuneParams.SizeIopsSec),
					domainName,
					disk.Name)
			}
		}
	}

	// Report network interface statistics.
	for _, iface := range stat.Net {
		var SourceBridge string
		var VirtualInterface string
		// Additional info for ovs network
		for _, net := range desc.Devices.Interfaces {
			if net.Target.Device == iface.Name {
				SourceBridge = net.Source.Bridge
				VirtualInterface = net.Virtualport.Parameters.InterfaceID
				break
			}
		}
		if SourceBridge != "" || VirtualInterface != "" {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainMetaInterfacesDesc,
				prometheus.GaugeValue,
				float64(1),
				domainName,
				SourceBridge,
				iface.Name,
				VirtualInterface)
		}
		if iface.RxBytesSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainInterfaceRxBytesDesc,
				prometheus.CounterValue,
				float64(iface.RxBytes),
				domainName,
				iface.Name)
		}
		if iface.RxPktsSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainInterfaceRxPacketsDesc,
				prometheus.CounterValue,
				float64(iface.RxPkts),
				domainName,
				iface.Name)
		}
		if iface.RxErrsSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainInterfaceRxErrsDesc,
				prometheus.CounterValue,
				float64(iface.RxErrs),
				domainName,
				iface.Name)
		}
		if iface.RxDropSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainInterfaceRxDropDesc,
				prometheus.CounterValue,
				float64(iface.RxDrop),
				domainName,
				iface.Name)
		}
		if iface.TxBytesSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainInterfaceTxBytesDesc,
				prometheus.CounterValue,
				float64(iface.TxBytes),
				domainName,
				iface.Name)
		}
		if iface.TxPktsSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainInterfaceTxPacketsDesc,
				prometheus.CounterValue,
				float64(iface.TxPkts),
				domainName,
				iface.Name)
		}
		if iface.TxErrsSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainInterfaceTxErrsDesc,
				prometheus.CounterValue,
				float64(iface.TxErrs),
				domainName,
				iface.Name)
		}
		if iface.TxDropSet {
			ch <- prometheus.MustNewConstMetric(
				libvirtDomainInterfaceTxDropDesc,
				prometheus.CounterValue,
				float64(iface.TxDrop),
				domainName,
				iface.Name)
		}
	}

	// Collect Memory Stats
	memorystat, err := stat.Domain.MemoryStats(11, 0)
	var MemoryStats libvirtSchema.VirDomainMemoryStats
	var usedPercent float64
	if err == nil {
		MemoryStats = memoryStatCollect(&memorystat)
		if MemoryStats.Usable != 0 && MemoryStats.Available != 0 {
			usedPercent = (float64(MemoryStats.Available) - float64(MemoryStats.Usable)) / (float64(MemoryStats.Available) / float64(100))
		}

	}
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainMemoryStatMajorFaultTotalDesc,
		prometheus.CounterValue,
		float64(MemoryStats.MajorFault),
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainMemoryStatMinorFaultTotalDesc,
		prometheus.CounterValue,
		float64(MemoryStats.MinorFault),
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainMemoryStatUnusedBytesDesc,
		prometheus.GaugeValue,
		float64(MemoryStats.Unused)*1024,
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainMemoryStatAvailableBytesDesc,
		prometheus.GaugeValue,
		float64(MemoryStats.Available)*1024,
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainMemoryStatActualBaloonBytesDesc,
		prometheus.GaugeValue,
		float64(MemoryStats.ActualBalloon)*1024,
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainMemoryStatRssBytesDesc,
		prometheus.GaugeValue,
		float64(MemoryStats.Rss)*1024,
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainMemoryStatUsableBytesDesc,
		prometheus.GaugeValue,
		float64(MemoryStats.Usable)*1024,
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainMemoryStatDiskCachesBytesDesc,
		prometheus.GaugeValue,
		float64(MemoryStats.DiskCaches)*1024,
		domainName)
	ch <- prometheus.MustNewConstMetric(
		libvirtDomainMemoryStatUsedPercentDesc,
		prometheus.GaugeValue,
		float64(usedPercent),
		domainName)

	return nil
}

// Collect Storage pool stats
func CollectStoragePool(ch chan<- prometheus.Metric, pool libvirt.StoragePool) error {
	// Refresh pool
	err := pool.Refresh(0)
	if err != nil {
		return err
	}
	pool_name, err := pool.GetName()
	if err != nil {
		return err
	}
	pool_info, err := pool.GetInfo()
	if err != nil {
		return err
	}
	// Send metrics to channel
	ch <- prometheus.MustNewConstMetric(
		libvirtPoolInfoCapacity,
		prometheus.GaugeValue,
		float64(pool_info.Capacity),
		pool_name)
	ch <- prometheus.MustNewConstMetric(
		libvirtPoolInfoAllocation,
		prometheus.GaugeValue,
		float64(pool_info.Allocation),
		pool_name)
	ch <- prometheus.MustNewConstMetric(
		libvirtPoolInfoAvailable,
		prometheus.GaugeValue,
		float64(pool_info.Available),
		pool_name)
	return nil
}

// CollectFromLibvirt obtains Prometheus metrics from all domains in a
// libvirt setup.
func CollectFromLibvirt(ch chan<- prometheus.Metric, uri string, logger log.Logger) error {
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return err
	}
	defer conn.Close()

	hypervisorVersionNum, err := conn.GetVersion() // virConnectGetVersion, hypervisor running, e.g. QEMU
	if err != nil {
		return err
	}
	hypervisorVersion := fmt.Sprintf("%d.%d.%d", hypervisorVersionNum/1000000%1000, hypervisorVersionNum/1000%1000, hypervisorVersionNum%1000)

	libvirtdVersionNum, err := conn.GetLibVersion() // virConnectGetLibVersion, libvirt daemon running
	if err != nil {
		return err
	}
	libvirtdVersion := fmt.Sprintf("%d.%d.%d", libvirtdVersionNum/1000000%1000, libvirtdVersionNum/1000%1000, libvirtdVersionNum%1000)

	libraryVersionNum, err := libvirt.GetVersion() // virGetVersion, version of libvirt (dynamic) library used by this binary (exporter), not the daemon version
	if err != nil {
		return err
	}
	libraryVersion := fmt.Sprintf("%d.%d.%d", libraryVersionNum/1000000%1000, libraryVersionNum/1000%1000, libraryVersionNum%1000)

	// Get all host processes in order to get the VM Pid.
	processes = utils.GetProcessList(*procFSPath)

	ch <- prometheus.MustNewConstMetric(
		libvirtVersionsInfoDesc,
		prometheus.GaugeValue,
		1.0,
		hypervisorVersion,
		libvirtdVersion,
		libraryVersion)

	stats, err := conn.GetAllDomainStats([]*libvirt.Domain{}, libvirt.DOMAIN_STATS_STATE|libvirt.DOMAIN_STATS_CPU_TOTAL|
		libvirt.DOMAIN_STATS_INTERFACE|libvirt.DOMAIN_STATS_BALLOON|libvirt.DOMAIN_STATS_BLOCK|
		libvirt.DOMAIN_STATS_PERF|libvirt.DOMAIN_STATS_VCPU,
		//libvirt.CONNECT_GET_ALL_DOMAINS_STATS_NOWAIT, // maybe in future
		libvirt.CONNECT_GET_ALL_DOMAINS_STATS_RUNNING|libvirt.CONNECT_GET_ALL_DOMAINS_STATS_SHUTOFF)
	defer func(stats []libvirt.DomainStats) {
		for _, stat := range stats {
			stat.Domain.Free()
		}
	}(stats)
	if err != nil {
		return err
	}
	for _, stat := range stats {
		err = CollectDomain(ch, stat, logger)
		if err != nil {
			return err
		}
	}

	// Collect pool info
	pools, err := conn.ListAllStoragePools(libvirt.CONNECT_LIST_STORAGE_POOLS_ACTIVE)
	if err != nil {
		return err
	}
	for _, pool := range pools {
		err = CollectStoragePool(ch, pool)
		pool.Free()
		if err != nil {
			return err
		}
	}
	return nil
}

func memoryStatCollect(memorystat *[]libvirt.DomainMemoryStat) libvirtSchema.VirDomainMemoryStats {
	var MemoryStats libvirtSchema.VirDomainMemoryStats
	for _, domainmemorystat := range *memorystat {
		switch tag := domainmemorystat.Tag; tag {
		case 2:
			MemoryStats.MajorFault = domainmemorystat.Val
		case 3:
			MemoryStats.MinorFault = domainmemorystat.Val
		case 4:
			MemoryStats.Unused = domainmemorystat.Val
		case 5:
			MemoryStats.Available = domainmemorystat.Val
		case 6:
			MemoryStats.ActualBalloon = domainmemorystat.Val
		case 7:
			MemoryStats.Rss = domainmemorystat.Val
		case 8:
			MemoryStats.Usable = domainmemorystat.Val
		case 10:
			MemoryStats.DiskCaches = domainmemorystat.Val
		}
	}
	return MemoryStats
}

// LibvirtExporter implements a Prometheus exporter for libvirt state.
type LibvirtExporter struct {
	uri    string
	logger log.Logger
}

// NewLibvirtExporter creates a new Prometheus exporter for libvirt.
func NewLibvirtExporter(uri string, logger log.Logger) (*LibvirtExporter, error) {
	return &LibvirtExporter{
		uri:    uri,
		logger: logger,
	}, nil
}

// Describe returns metadata for all Prometheus metrics that may be exported.
func (e *LibvirtExporter) Describe(ch chan<- *prometheus.Desc) {
	// Status and versions
	ch <- libvirtUpDesc
	ch <- libvirtVersionsInfoDesc

	// Pool info
	ch <- libvirtPoolInfoCapacity
	ch <- libvirtPoolInfoAllocation
	ch <- libvirtPoolInfoAvailable

	// Domain info
	ch <- libvirtDomainInfoMetaDesc
	ch <- libvirtDomainInfoMaxMemBytesDesc
	ch <- libvirtDomainInfoMemoryUsageBytesDesc
	ch <- libvirtDomainInfoNrVirtCPUDesc
	ch <- libvirtDomainInfoCPUTimeDesc
	ch <- libvirtDomainInfoVirDomainState

	// VCPU info
	ch <- libvirtDomainVcpuStateDesc
	ch <- libvirtDomainVcpuTimeDesc
	ch <- libvirtDomainVcpuDelayDesc
	ch <- libvirtDomainVcpuCPUDesc
	ch <- libvirtDomainVcpuWaitDesc

	// Domain block stats
	ch <- libvirtDomainMetaBlockDesc
	ch <- libvirtDomainBlockRdBytesDesc
	ch <- libvirtDomainBlockRdReqDesc
	ch <- libvirtDomainBlockRdTotalTimeSecondsDesc
	ch <- libvirtDomainBlockWrBytesDesc
	ch <- libvirtDomainBlockWrReqDesc
	ch <- libvirtDomainBlockWrTotalTimesDesc
	ch <- libvirtDomainBlockFlushReqDesc
	ch <- libvirtDomainBlockFlushTotalTimeSecondsDesc
	ch <- libvirtDomainBlockAllocationDesc
	ch <- libvirtDomainBlockCapacityBytesDesc
	ch <- libvirtDomainBlockPhysicalSizeBytesDesc

	// Domain net interfaces stats
	ch <- libvirtDomainMetaInterfacesDesc
	ch <- libvirtDomainInterfaceRxBytesDesc
	ch <- libvirtDomainInterfaceRxPacketsDesc
	ch <- libvirtDomainInterfaceRxErrsDesc
	ch <- libvirtDomainInterfaceRxDropDesc
	ch <- libvirtDomainInterfaceTxBytesDesc
	ch <- libvirtDomainInterfaceTxPacketsDesc
	ch <- libvirtDomainInterfaceTxErrsDesc
	ch <- libvirtDomainInterfaceTxDropDesc

	// Domain memory stats
	ch <- libvirtDomainMemoryStatMajorFaultTotalDesc
	ch <- libvirtDomainMemoryStatMinorFaultTotalDesc
	ch <- libvirtDomainMemoryStatUnusedBytesDesc
	ch <- libvirtDomainMemoryStatAvailableBytesDesc
	ch <- libvirtDomainMemoryStatActualBaloonBytesDesc
	ch <- libvirtDomainMemoryStatRssBytesDesc
	ch <- libvirtDomainMemoryStatUsableBytesDesc
	ch <- libvirtDomainMemoryStatDiskCachesBytesDesc
}

// Collect scrapes Prometheus metrics from libvirt.
func (e *LibvirtExporter) Collect(ch chan<- prometheus.Metric) {
	err := CollectFromLibvirt(ch, e.uri, e.logger)
	if err == nil {
		ch <- prometheus.MustNewConstMetric(
			libvirtUpDesc,
			prometheus.GaugeValue,
			1.0)
	} else {
		_ = level.Error(e.logger).Log("err", "failed to scrape metrics", "uri", e.uri, "msg", err)
		ch <- prometheus.MustNewConstMetric(
			libvirtUpDesc,
			prometheus.GaugeValue,
			0.0)
	}
}

// ConnectURI defines a type for driver URIs for libvirt
// the defined constants are *not* exhaustive as there are also options
// e.g. to connect remote via SSH
type ConnectURI string

// See also https://libvirt.org/html/libvirt-libvirt-host.html#virConnectOpen
const (
	// QEMUSystem connects to a QEMU system mode daemon
	QEMUSystem ConnectURI = "qemu:///system"
	// QEMUSession connects to a QEMU session mode daemon (unprivileged)
	QEMUSession ConnectURI = "qemu:///session"
	// XenSystem connects to a Xen system mode daemon
	XenSystem ConnectURI = "xen:///system"
	//TestDefault connect to default mock driver
	TestDefault ConnectURI = "test:///default"
)

func main() {
	var libvirtURI = kingpin.Flag("libvirt.uri",
		fmt.Sprintf("Libvirt URI to extract metrics, available value: %s (default), %s, %s and %s ",
			QEMUSystem, QEMUSession, XenSystem, TestDefault),
	).Default(string(QEMUSystem)).String()

	metricsPath := kingpin.Flag(
		"web.telemetry-path", "Path under which to expose metrics",
	).Default("/metrics").String()
	toolkitFlags := webflag.AddFlags(kingpin.CommandLine, ":9177")

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("libvirt_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	_ = level.Info(logger).Log("msg", "Starting libvirt_exporter", "version", version.Info())
	_ = level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())

	errorsMap = make(map[string]struct{})

	exporter, err := NewLibvirtExporter(*libvirtURI, logger)
	if err != nil {
		panic(err)
	}

	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.Handler())
	if *metricsPath != "/" {
		landingCnf := web.LandingConfig{
			Name:        "Libvirt Exporter",
			Description: "Prometheus Libvirt Exporter",
			Version:     version.Info(),
			Links: []web.LandingLinks{
				{
					Address: *metricsPath,
					Text:    "Metrics",
				},
			},
		}
		landingPage, err := web.NewLandingPage(landingCnf)
		if err != nil {
			_ = level.Error(logger).Log("err", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}

	srv := &http.Server{}
	if err = web.ListenAndServe(srv, toolkitFlags, logger); err != nil {
		_ = level.Error(logger).Log("err", err)
		os.Exit(1)
	}
}
