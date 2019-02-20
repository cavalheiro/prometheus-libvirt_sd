# Prometheus libvirt Service Discovery

Prometheus offers a variety of service discovery options for discovering scrape targets, but those do not include libvirt. This tool connects to libvirt hypervisors and generates Prometheus scrape target configurations, taking advantage of the [file-based service discovery](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#%3Cfile_sd_config) mechanism provided by Prometheus.

You can define a set of rules on how to group and label libvirt guests. Those rules are defined in a [configuration file](prometheus-libvirt_sd.yml) and support _regular expressions_ using the same general syntax used by Perl, Python, and other languages. Complete rules are described at https://golang.org/s/re2syntax.

**Currently, the guest FQDN's are resolved via a "guessing" method** that combines each guest's VM name with the hypervisor's domain name. _This only works under strict conditions where the DNS naming of guests follows this convention_. There is work in progress development to support real name resolution using libvirt guest agent.

Example rule:
```yaml
- labels: { job: libvirt }
  domains:
    - match: .*
      ports: [9100]
```

Example output:
```yaml
- targets:
  - guest-1.local:9100
  - guest-2.local:9100
  - guest-3.local:9100
  labels:
    hypervisor: hypervisor.local
    job: libvirt
```

## Prometheus Configuration

One result file will be generated per each hypervisor defined in the configuration settings, named after the hypervisor FQDN. Please remember to adjust your `prometheus.yml` configuration file to use the file service discovery mechanism and point it to the output location of this tool.

Example configuration section of prometheus.yml:
```yaml
- job_name: 'overwritten-default'
  file_sd_configs:
   - files: ['/data/prometheus/scrape-config/*.yml']
```

## License

This project has been released under the MIT license. Please see the LICENSE.md file for more details.
