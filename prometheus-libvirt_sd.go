package main

import (
	"flag"
	"io/ioutil"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	libvirt "github.com/libvirt/libvirt-go"
	// "github.com/libvirt/libvirt-go-xml"
	"gopkg.in/yaml.v2"
)

// Constants

const DEFAULT_CONFIG_FILE = "prometheus-libvirt_sd.yml"

// Structures

type Group struct {
	Labels map[string]string
	Domains []Domain
}

type Domain struct {
	Match string
	Ports []string
	Labels map[string]string
}

type Config struct {
	OutputDir string
	PollingInterval int
	Hosts []string
	Groups []Group
}

// Result structure

type PromScrapeGroup struct {
	Targets []string
	Labels  map[string]string
}

var config Config

// Given a list of domains obtained from an hypervisor,
// find the ones that match configured groups
func findMatchingDomains(domainList []libvirt.Domain, hvDomain string,
	domainExpr string) ([]string) {
	result := []string{}
	for _, domainObj := range domainList {
		domainName, _ := domainObj.GetName()
		fqdn := domainName + hvDomain
		match, _ := regexp.MatchString(domainExpr, fqdn)
		if match {
			result = append(result, domainName)
		}
	}
	return result
}

// Generate scrape configuration for a given Libvirt hypervisor
func queryLibvirtHypervisor(uri string) (int, error) {
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return -1, err
	}

	// Get all domains from libvirt hypervisor
	hvDomain := getHypervisorDomainName(conn)
	promConfig := []PromScrapeGroup{}
	domainList, _ := conn.ListAllDomains(0)

	// Close connections once we finish
	defer func() {
		for _, domainObj := range domainList {
			domainObj.Free()
		}
		conn.Close()
	}()

	for _, groupConfig := range config.Groups {
		for _, domainConfig := range groupConfig.Domains {
			// Declare base structure for prom scrape group
			promScrapeGroup := PromScrapeGroup{
				Targets: []string{},
				Labels: map[string]string{
					"hypervisor" : hvDomain[0]}}
			// Check if the domain config iteratee exists on hypervisor
			matchedDomains := findMatchingDomains(domainList, hvDomain[1], domainConfig.Match)
			if len(matchedDomains) > 0 {
				// Add group labels
				for k, v := range groupConfig.Labels {
				    promScrapeGroup.Labels[k] = v
				}
				// Add domain labels
				for k, v := range domainConfig.Labels {
				    promScrapeGroup.Labels[k] = v
				}
				// Add scrape targets and ports
				for _, dom :=  range matchedDomains {
					for _, port := range domainConfig.Ports {
						promScrapeGroup.Targets = append(promScrapeGroup.Targets, dom + "." + hvDomain[1] + ":" + port)
					}
				}
				promConfig = append(promConfig, promScrapeGroup)
			}
		}
	}
	return writePromConfig(hvDomain[0], promConfig)
}


// Given a libvirt connection, return the hypervisor domain name
func getHypervisorDomainName(conn *libvirt.Connect) []string {
	hostName, _ := conn.GetHostname()
	re, _ := regexp.Compile(`\w+\.(.*)`)
	match := re.FindStringSubmatch(hostName)
	domain := "local"
	if len(match) > 1 {
		domain = match[1]
	}
	return []string{hostName, domain}
}

// Generate the yaml files for Prometheus based on a scrape config structure
func writePromConfig(hvDomain string, promConfig []PromScrapeGroup) (int, error) {
	ymlPromConfig, _ := yaml.Marshal(promConfig)
	err := ioutil.WriteFile(config.OutputDir+"/"+hvDomain+".yml", []byte(ymlPromConfig), 0644)
	fmt.Printf("Scrape config for %v: %v", hvDomain, string(ymlPromConfig))
	if err != nil {
		return -1, err
	}
	return len(promConfig), nil
}

// Error handler
func fatalErrorHandler(e error, msg string) {
	if e != nil {
		fmt.Printf("ERROR: %s\n", e.Error())
		fmt.Printf("ERROR: %s\n", msg)
		os.Exit(1)
	}
}

func main() {
	// Parse command line arguments
	var (
		configFile = flag.String("config", DEFAULT_CONFIG_FILE, "Path to config file")
	)
	flag.Parse()

	// Set defaults
	config = Config{PollingInterval: 120, OutputDir: "/tmp"}

	// Load configuration file
	dat, err := ioutil.ReadFile(*configFile)
	fatalErrorHandler(err, "Unable to read configuration file - please specify the correct location using --config=file.yml")
	err = yaml.Unmarshal([]byte(dat), &config)
	fatalErrorHandler(err, "Unable to parse configuration file")
	// Trace.Printf("\n%v\n", config)

	// Output some info about supplied config
	fmt.Printf("Using config file: %v\n", *configFile)
	fmt.Printf("\tpolling interval: %d seconds\n", config.PollingInterval)
	fmt.Printf("\toutput dir: %v\n", config.OutputDir)

	// Loop through list of configured hypervisors
	for {
		var wg sync.WaitGroup
		wg.Add(len(config.Hosts))
		for _, host := range config.Hosts {
			go func(host string) {
				defer wg.Done()
				fmt.Printf("Querying hypervisor %+v...\n", host)
				c, err := queryLibvirtHypervisor(host)
				if err != nil {
					fmt.Printf("ERROR: %v - %v+\n", host, err)
				} else {
					fmt.Printf("%v: scrape config updated (%d groups).\n", host, c)
				}
			}(host)
		}
		wg.Wait()
		if config.PollingInterval > 0 {
			time.Sleep(time.Duration(config.PollingInterval) * time.Second)
		} else {
			break
		}
	}
}
